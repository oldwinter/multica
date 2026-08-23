import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const twinExecutionKeys = {
  all: (wsId: string) => ["workspaces", wsId, "twin-execution"] as const,
  bindings: (wsId: string) => [...twinExecutionKeys.all(wsId), "bindings"] as const,
  metrics: (wsId: string) => [...twinExecutionKeys.all(wsId), "metrics"] as const,
  taskContext: (wsId: string, taskId: string) =>
    [...twinExecutionKeys.all(wsId), "tasks", taskId, "context"] as const,
};

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
