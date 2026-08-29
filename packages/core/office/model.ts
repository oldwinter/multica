import {
  deriveAgentAvailability,
  deriveWorkloadDetail,
} from "../agents/derive-presence";
import { issueStatusCategory } from "../issues/status-category";
import type { AgentTask } from "../types";
import type {
  DeriveOfficeSnapshotInput,
  OfficeAgent,
  OfficeIssue,
  OfficeSnapshot,
  OfficeSquad,
  OfficeSquadMemberPreview,
} from "./types";
import { OFFICE_LIMITS } from "./types";

const ACTIVE_TASK_STATUSES = new Set([
  "queued",
  "dispatched",
  "waiting_local_directory",
  "running",
]);

const KNOWN_TASK_STATUSES = new Set([
  ...ACTIVE_TASK_STATUSES,
  "completed",
  "failed",
  "cancelled",
]);

function memberKind(memberType: string): OfficeSquadMemberPreview["kind"] {
  if (memberType === "agent" || memberType === "member") return memberType;
  return "unknown";
}

function exactOverflow(total: number, limit: number): number {
  return Math.max(0, total - limit);
}

export function buildOfficeSnapshot(
  input: DeriveOfficeSnapshotInput,
): OfficeSnapshot {
  const liveAgents = input.agents
    .filter((agent) => agent.archived_at === null)
    .sort((left, right) => left.id.localeCompare(right.id));
  const liveAgentIds = new Set(liveAgents.map((agent) => agent.id));

  const runtimesById = new Map(
    input.runtimes.kind === "available"
      ? input.runtimes.value.map((runtime) => [runtime.id, runtime] as const)
      : [],
  );
  const tasksByAgent = new Map<string, AgentTask[]>();
  const activeIssueAgents = new Map<string, Set<string>>();
  const activeIssueIdsByAgent = new Map<string, Set<string>>();
  const unknownTaskAgentIds = new Set<string>();

  if (input.tasks.kind === "available") {
    for (const task of input.tasks.value) {
      if (!liveAgentIds.has(task.agent_id)) continue;
      const agentTasks = tasksByAgent.get(task.agent_id);
      if (agentTasks) agentTasks.push(task);
      else tasksByAgent.set(task.agent_id, [task]);

      if (!KNOWN_TASK_STATUSES.has(task.status)) {
        unknownTaskAgentIds.add(task.agent_id);
        continue;
      }
      if (!ACTIVE_TASK_STATUSES.has(task.status) || task.issue_id === "") continue;

      const executingAgents = activeIssueAgents.get(task.issue_id);
      if (executingAgents) executingAgents.add(task.agent_id);
      else activeIssueAgents.set(task.issue_id, new Set([task.agent_id]));

      const agentIssues = activeIssueIdsByAgent.get(task.agent_id);
      if (agentIssues) agentIssues.add(task.issue_id);
      else activeIssueIdsByAgent.set(task.agent_id, new Set([task.issue_id]));
    }
  }

  const agents: OfficeAgent[] = liveAgents.map((agent) => {
    const availability: OfficeAgent["availability"] =
      input.runtimes.kind === "unavailable"
        ? {
            kind: "unknown",
            reason: input.runtimes.reason ?? "unavailable",
          }
        : (() => {
            const value = deriveAgentAvailability(
              runtimesById.get(agent.runtime_id) ?? null,
              input.nowMs,
            );
            return {
              kind: "known",
              value: value === "archived" ? "offline" : value,
            };
          })();

    let workload: OfficeAgent["workload"];
    if (
      input.tasks.kind === "unavailable" ||
      unknownTaskAgentIds.has(agent.id)
    ) {
      workload = {
        kind: "unknown",
        reason:
          input.tasks.kind === "unavailable"
            ? input.tasks.reason ?? "unavailable"
            : "unavailable",
        capacity: agent.max_concurrent_tasks,
      };
    } else {
      const detail = deriveWorkloadDetail(
        tasksByAgent.get(agent.id) ?? [],
      );
      workload = {
        kind: "known",
        value: detail.workload,
        runningCount: detail.runningCount,
        queuedCount: detail.queuedCount,
        capacity: agent.max_concurrent_tasks,
      };
    }

    return {
      id: agent.id,
      name: agent.name,
      avatarUrl: agent.avatar_url,
      description: agent.description,
      availability,
      workload,
      activeIssueIds: [...(activeIssueIdsByAgent.get(agent.id) ?? [])].sort(),
    };
  });

  const squads: OfficeSquad[] =
    input.squads.kind === "available"
      ? input.squads.value
          .filter((squad) => squad.archived_at === null)
          .map((squad) => ({
            id: squad.id,
            name: squad.name,
            description: squad.description,
            avatarUrl: squad.avatar_url,
            leaderAgentId: squad.leader_id,
            memberCount:
              squad.member_count ?? squad.member_preview?.length ?? 0,
            memberPreview: (squad.member_preview ?? []).map((member) => ({
              kind: memberKind(member.member_type),
              id: member.member_id,
              role: member.role,
            })),
          }))
          .sort((left, right) => left.id.localeCompare(right.id))
      : [];

  const activeIssueIds = [...activeIssueAgents.keys()].sort();
  const requestedIssueIds = new Set(
    activeIssueIds.slice(0, OFFICE_LIMITS.issueBriefs),
  );
  const issueBriefsById = new Map(
    input.issueBriefs.kind === "available"
      ? input.issueBriefs.value
          .filter((issue) => requestedIssueIds.has(issue.id))
          .map((issue) => [issue.id, issue] as const)
      : [],
  );

  const activeIssues: OfficeIssue[] = activeIssueIds.map((id, index) => {
    const executingAgentIds = [...(activeIssueAgents.get(id) ?? [])].sort();
    if (index >= OFFICE_LIMITS.issueBriefs) {
      return {
        kind: "unresolved",
        id,
        reason: "brief-limit",
        executingAgentIds,
      };
    }
    if (input.issueBriefs.kind === "unavailable") {
      return {
        kind: "unresolved",
        id,
        reason:
          input.issueBriefs.reason === "loading" ? "loading" : "unavailable",
        executingAgentIds,
      };
    }
    const issue = issueBriefsById.get(id);
    if (!issue) {
      return {
        kind: "unresolved",
        id,
        reason: "not-returned",
        executingAgentIds,
      };
    }
    return {
      kind: "resolved",
      id,
      identifier: issue.identifier,
      title: issue.title,
      status: issue.status,
      statusCategory: issueStatusCategory(issue),
      assignedSquadId:
        issue.assignee_type === "squad" ? issue.assignee_id : null,
      executingAgentIds,
    };
  });

  return {
    agents,
    squads,
    activeIssues,
    overflow: {
      agents: exactOverflow(agents.length, input.limits.agents),
      squads: exactOverflow(squads.length, input.limits.squads),
      activeIssues: exactOverflow(
        activeIssues.length,
        input.limits.activeIssues,
      ),
    },
  };
}
