import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { AgentTask, Issue } from "../types";
import { OFFICE_LIMITS } from "./types";

export const officeKeys = {
  all: (wsId: string) => ["workspaces", wsId, "office"] as const,
  issueBriefsAll: (wsId: string) =>
    [...officeKeys.all(wsId), "issue-briefs"] as const,
  issueBriefs: (wsId: string, issueIds: readonly string[]) =>
    [
      ...officeKeys.issueBriefsAll(wsId),
      normalizeIssueIds(issueIds),
    ] as const,
};

function normalizeIssueIds(issueIds: readonly string[]): string[] {
  return [...new Set(issueIds.filter((id) => id !== ""))].sort();
}

function isActiveTaskStatus(status: string): boolean {
  return (
    status === "queued" ||
    status === "dispatched" ||
    status === "waiting_local_directory" ||
    status === "running"
  );
}

export function collectOfficeIssueBriefIds(
  tasks: readonly AgentTask[],
  limit: number = OFFICE_LIMITS.issueBriefs,
): string[] {
  const issueIds = new Set<string>();
  for (const task of tasks) {
    if (task.issue_id !== "" && isActiveTaskStatus(task.status)) {
      issueIds.add(task.issue_id);
    }
  }
  return [...issueIds].sort().slice(0, limit);
}

export function officeIssueBriefsOptions(
  wsId: string,
  issueIds: readonly string[],
) {
  const requestedIds = normalizeIssueIds(issueIds).slice(
    0,
    OFFICE_LIMITS.issueBriefs,
  );
  return queryOptions<Issue[]>({
    queryKey: officeKeys.issueBriefs(wsId, requestedIds),
    queryFn: async () => {
      if (requestedIds.length === 0) return [];
      const requested = new Set(requestedIds);
      const response = await api.listIssues({
        workspace_id: wsId,
        ids: requestedIds,
        limit: requestedIds.length,
      });
      return response.issues.filter((issue) => requested.has(issue.id));
    },
    enabled: wsId !== "" && requestedIds.length > 0,
    staleTime: 30 * 1000,
  });
}
