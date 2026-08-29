"use client";

import { useQuery } from "@tanstack/react-query";
import { agentTaskSnapshotOptions } from "../agents/queries";
import type { AgentTask } from "../types";

const EMPTY_TASKS: readonly AgentTask[] = [];

export interface OfficeTaskCache {
  readonly tasks: readonly AgentTask[];
  readonly isFetching: boolean;
  readonly dataUpdatedAt: number;
}

export function useOfficeTaskCache(wsId: string): OfficeTaskCache {
  const query = useQuery({
    ...agentTaskSnapshotOptions(wsId),
    enabled: wsId !== "",
  });
  return {
    tasks: query.data ?? EMPTY_TASKS,
    isFetching: query.isFetching,
    dataUpdatedAt: query.dataUpdatedAt,
  };
}
