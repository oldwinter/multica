"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { agentTaskSnapshotOptions } from "../agents/queries";
import { issueDetailOptions } from "../issues/queries";
import { issueStatusCategory } from "../issues/status-category";
import { runtimeListOptions } from "../runtimes/queries";
import type {
  AgentRuntime,
  AgentTask,
  Issue,
  Squad,
} from "../types";
import {
  agentListOptions,
  memberListOptions,
  squadListOptions,
  squadMemberStatusOptions,
} from "../workspace/queries";
import { buildOfficeSnapshot } from "./model";
import {
  collectOfficeIssueBriefIds,
  officeIssueBriefsOptions,
} from "./queries";
import {
  OFFICE_LIMITS,
  type OfficeDataGap,
  type OfficeInspector,
  type OfficeIssue,
  type OfficeModel,
  type OfficeSource,
  type OfficeSquadMembers,
  type OfficeSubjectRef,
} from "./types";

const PRESENCE_TICK_MS = 30_000;

function usePresenceNow(): number {
  const [nowMs, setNowMs] = useState(() => Date.now());
  useEffect(() => {
    const interval = setInterval(() => setNowMs(Date.now()), PRESENCE_TICK_MS);
    return () => clearInterval(interval);
  }, []);
  return nowMs;
}

function sourceOf<T>(data: T | undefined, isPending: boolean): OfficeSource<T> {
  if (data !== undefined) return { kind: "available", value: data };
  return {
    kind: "unavailable",
    reason: isPending ? "loading" : "unavailable",
  };
}

function resolvedIssue(
  issue: Issue,
  executingAgentIds: readonly string[],
): OfficeIssue {
  return {
    kind: "resolved",
    id: issue.id,
    identifier: issue.identifier,
    title: issue.title,
    status: issue.status,
    statusCategory: issueStatusCategory(issue),
    assignedSquadId:
      issue.assignee_type === "squad" ? issue.assignee_id : null,
    executingAgentIds,
  };
}

export function useOfficeModel(input: {
  readonly wsId: string;
  readonly selected: OfficeSubjectRef | null;
}): OfficeModel {
  const agentsQuery = useQuery({
    ...agentListOptions(input.wsId),
    enabled: input.wsId !== "",
  });
  const runtimesQuery = useQuery({
    ...runtimeListOptions(input.wsId),
    enabled: input.wsId !== "",
  });
  const tasksQuery = useQuery({
    ...agentTaskSnapshotOptions(input.wsId),
    enabled: input.wsId !== "",
  });
  const squadsQuery = useQuery({
    ...squadListOptions(input.wsId),
    enabled: input.wsId !== "",
  });
  const liveAgentIds = useMemo(
    () =>
      new Set(
        (agentsQuery.data ?? [])
          .filter((agent) => agent.archived_at === null)
          .map((agent) => agent.id),
      ),
    [agentsQuery.data],
  );
  const issueIds = useMemo(
    () =>
      collectOfficeIssueBriefIds(
        (tasksQuery.data ?? []).filter((task) =>
          liveAgentIds.has(task.agent_id),
        ),
      ),
    [liveAgentIds, tasksQuery.data],
  );
  const issueBriefsQuery = useQuery(
    officeIssueBriefsOptions(input.wsId, issueIds),
  );

  const selectedSquadId =
    input.selected?.kind === "squad" ? input.selected.id : "";
  const squadStatusQuery = useQuery({
    ...squadMemberStatusOptions(input.wsId, selectedSquadId),
    enabled: input.wsId !== "" && selectedSquadId !== "",
  });
  const needsHumanNames =
    squadStatusQuery.data?.members.some(
      (member) => member.member_type === "member",
    ) === true;
  const membersQuery = useQuery({
    ...memberListOptions(input.wsId),
    enabled: input.wsId !== "" && selectedSquadId !== "" && needsHumanNames,
  });

  const nowMs = usePresenceNow();
  const snapshot = useMemo(() => {
    if (!agentsQuery.data) return null;
    const issueBriefs: OfficeSource<readonly Issue[]> =
      issueIds.length === 0
        ? { kind: "available", value: [] }
        : sourceOf(issueBriefsQuery.data, issueBriefsQuery.isPending);
    return buildOfficeSnapshot({
      nowMs,
      agents: agentsQuery.data,
      runtimes: sourceOf<readonly AgentRuntime[]>(
        runtimesQuery.data,
        runtimesQuery.isPending,
      ),
      tasks: sourceOf<readonly AgentTask[]>(
        tasksQuery.data,
        tasksQuery.isPending,
      ),
      squads: sourceOf<readonly Squad[]>(
        squadsQuery.data,
        squadsQuery.isPending,
      ),
      issueBriefs,
      limits: OFFICE_LIMITS,
    });
  }, [
    agentsQuery.data,
    runtimesQuery.data,
    runtimesQuery.isPending,
    tasksQuery.data,
    tasksQuery.isPending,
    squadsQuery.data,
    squadsQuery.isPending,
    issueIds,
    issueBriefsQuery.data,
    issueBriefsQuery.isPending,
    nowMs,
  ]);

  const selectedIssue =
    input.selected?.kind === "issue"
      ? snapshot?.activeIssues.find((issue) => issue.id === input.selected?.id)
      : undefined;
  const unresolvedSelectedIssueId =
    selectedIssue?.kind === "unresolved" ? selectedIssue.id : "";
  const issueDetailEnabled =
    input.wsId !== "" &&
    unresolvedSelectedIssueId !== "" &&
    selectedIssue?.kind === "unresolved" &&
    selectedIssue.reason !== "loading";
  const issueDetailQuery = useQuery({
    ...issueDetailOptions(input.wsId, unresolvedSelectedIssueId),
    enabled: issueDetailEnabled,
  });

  const retry = useCallback(async () => {
    const retries: Promise<unknown>[] = [
      agentsQuery.refetch(),
      runtimesQuery.refetch(),
      tasksQuery.refetch(),
      squadsQuery.refetch(),
    ];
    if (issueIds.length > 0) retries.push(issueBriefsQuery.refetch());
    if (selectedSquadId !== "") retries.push(squadStatusQuery.refetch());
    if (needsHumanNames) retries.push(membersQuery.refetch());
    if (issueDetailEnabled) retries.push(issueDetailQuery.refetch());
    await Promise.all(retries);
  }, [
    agentsQuery,
    runtimesQuery,
    tasksQuery,
    squadsQuery,
    issueIds.length,
    issueBriefsQuery,
    selectedSquadId,
    squadStatusQuery,
    needsHumanNames,
    membersQuery,
    issueDetailEnabled,
    issueDetailQuery,
  ]);

  if (!agentsQuery.data) {
    if (agentsQuery.isPending) return { kind: "loading" };
    return { kind: "unavailable", retry };
  }
  if (!snapshot) return { kind: "loading" };

  let inspector: OfficeInspector = { kind: "closed" };
  if (input.selected?.kind === "agent") {
    const agent = snapshot.agents.find((candidate) => candidate.id === input.selected?.id);
    inspector = agent
      ? { kind: "agent", agent }
      : { kind: "missing", subject: input.selected };
  } else if (input.selected?.kind === "issue") {
    const issue = snapshot.activeIssues.find(
      (candidate) => candidate.id === input.selected?.id,
    );
    const detail =
      issue?.kind === "unresolved" && issueDetailQuery.data
        ? resolvedIssue(issueDetailQuery.data, issue.executingAgentIds)
        : issue;
    inspector = detail
      ? { kind: "issue", issue: detail }
      : { kind: "missing", subject: input.selected };
  } else if (input.selected?.kind === "squad") {
    const squad = snapshot.squads.find(
      (candidate) => candidate.id === input.selected?.id,
    );
    let members: OfficeSquadMembers;
    if (squadStatusQuery.isPending) {
      members = { kind: "loading" };
    } else if (!squadStatusQuery.data) {
      members = { kind: "unavailable", retry: async () => void (await squadStatusQuery.refetch()) };
    } else if (needsHumanNames && membersQuery.isPending) {
      members = { kind: "loading" };
    } else if (needsHumanNames && !membersQuery.data) {
      members = {
        kind: "unavailable",
        retry: async () => void (await membersQuery.refetch()),
      };
    } else {
      const namesByMemberId = new Map(
        (membersQuery.data ?? []).map((member) => [member.id, member.name] as const),
      );
      const agentNamesById = new Map(
        snapshot.agents.map((agent) => [agent.id, agent.name] as const),
      );
      members = {
        kind: "ready",
        members: squadStatusQuery.data.members.map((member) => ({
          kind:
            member.member_type === "agent" || member.member_type === "member"
              ? member.member_type
              : "unknown",
          id: member.member_id,
          name:
            member.member_type === "agent"
              ? agentNamesById.get(member.member_id) ?? null
              : member.member_type === "member"
                ? namesByMemberId.get(member.member_id) ?? null
                : null,
          activeIssueIds: [
            ...new Set(member.active_issues.map((issue) => issue.issue_id)),
          ].sort(),
        })),
      };
    }
    inspector = squad
      ? { kind: "squad", squad, members }
      : { kind: "missing", subject: input.selected };
  }

  const gaps: OfficeDataGap[] = [];
  if (!runtimesQuery.data) gaps.push("availability");
  if (!tasksQuery.data) gaps.push("workload");
  if (!squadsQuery.data) gaps.push("squads");
  if (issueIds.length > 0 && !issueBriefsQuery.data) gaps.push("issue-briefs");
  if (selectedSquadId !== "" && squadStatusQuery.isError) {
    gaps.push("selected-squad");
  }
  if (needsHumanNames && membersQuery.isError) gaps.push("selected-squad");
  const stale = [
    agentsQuery,
    runtimesQuery,
    tasksQuery,
    squadsQuery,
    issueBriefsQuery,
    squadStatusQuery,
    membersQuery,
    issueDetailQuery,
  ].some((query) => query.isRefetchError);
  const refreshing = [
    agentsQuery,
    runtimesQuery,
    tasksQuery,
    squadsQuery,
    issueBriefsQuery,
    squadStatusQuery,
    membersQuery,
    issueDetailQuery,
  ].some((query) => query.isFetching);

  return {
    kind: "ready",
    snapshot,
    quality:
      stale
        ? { kind: "stale", gaps }
        : gaps.length > 0
          ? { kind: "partial", gaps }
          : { kind: "current", refreshing },
    inspector,
    retry,
  };
}
