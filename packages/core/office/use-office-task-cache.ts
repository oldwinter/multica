"use client";

import { useQuery } from "@tanstack/react-query";
import { agentTaskSnapshotOptions } from "../agents/queries";
import type { AgentTask } from "../types";
import type { OfficeTaskObservation } from "./continuity";

const EMPTY_OBSERVATIONS: readonly OfficeTaskObservation[] = [];

function taskObservation(task: AgentTask): OfficeTaskObservation | null {
  let status: OfficeTaskObservation["status"];
  switch (task.status) {
    case "queued":
    case "dispatched":
    case "waiting_local_directory":
      status = "queued-like";
      break;
    case "running":
    case "completed":
    case "failed":
      status = task.status;
      break;
    case "cancelled":
    default:
      return null;
  }
  return {
    taskId: task.id,
    agentId: task.agent_id,
    issueId: task.issue_id === "" ? null : task.issue_id,
    status,
  };
}

function taskObservations(
  tasks: readonly AgentTask[],
): readonly OfficeTaskObservation[] {
  return tasks.flatMap((task) => {
    const observation = taskObservation(task);
    return observation ? [observation] : [];
  });
}

export interface OfficeTaskCache {
  readonly observations: readonly OfficeTaskObservation[];
  readonly isFetching: boolean;
  readonly dataUpdatedAt: number;
}

export function useOfficeTaskCache(wsId: string): OfficeTaskCache {
  const query = useQuery({
    ...agentTaskSnapshotOptions(wsId),
    enabled: wsId !== "",
    select: taskObservations,
  });
  return {
    observations: query.data ?? EMPTY_OBSERVATIONS,
    isFetching: query.isFetching,
    dataUpdatedAt: query.dataUpdatedAt,
  };
}
