import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const twinExecutionKeys = {
  all: (wsId: string) => ["workspaces", wsId, "twin-execution"] as const,
  activation: (wsId: string) => [...twinExecutionKeys.all(wsId), "activation"] as const,
  bindings: (wsId: string) => [...twinExecutionKeys.all(wsId), "bindings"] as const,
  issueSelector: (wsId: string, search: string) =>
    [...twinExecutionKeys.all(wsId), "entity-selector", "issues", search] as const,
  metrics: (wsId: string) => [...twinExecutionKeys.all(wsId), "metrics"] as const,
  taskContext: (wsId: string, taskId: string) =>
    [...twinExecutionKeys.all(wsId), "tasks", taskId, "context"] as const,
};

export function twinActivationReadinessOptions(wsId: string) {
  return queryOptions({
    queryKey: twinExecutionKeys.activation(wsId),
    queryFn: () => api.getTwinActivationReadiness(),
    enabled: Boolean(wsId),
  });
}

export function twinIssueSelectorOptions(wsId: string, search: string) {
  const normalized = search.trim();
  return queryOptions({
    queryKey: twinExecutionKeys.issueSelector(wsId, normalized),
    queryFn: ({ signal }) => api.searchIssues({
      q: normalized,
      limit: 30,
      include_closed: true,
      signal,
    }).then((response) => response.issues),
    enabled: Boolean(wsId) && normalized.length > 0,
  });
}

export function twinBindingsOptions(wsId: string) {
  return queryOptions({
    queryKey: twinExecutionKeys.bindings(wsId),
    queryFn: () => api.getTwinBindings(),
    enabled: Boolean(wsId),
  });
}

export function twinExecutionMetricsOptions(wsId: string) {
  return queryOptions({
    queryKey: twinExecutionKeys.metrics(wsId),
    queryFn: () => api.getTwinExecutionMetrics(),
    enabled: Boolean(wsId),
  });
}

export function twinTaskContextOptions(wsId: string, taskId: string) {
  return queryOptions({
    queryKey: twinExecutionKeys.taskContext(wsId, taskId),
    queryFn: () => api.getTwinTaskContext(taskId),
    enabled: Boolean(wsId) && Boolean(taskId),
  });
}
