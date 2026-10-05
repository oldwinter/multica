import { useMutation } from "@tanstack/react-query";
import { api } from "../api";
import { taskRunReviewKeys } from "./queries";
import type { CreateTaskRunReviewInput } from "./types";

export function useCreateTaskRunReview(wsId: string, taskId: string) {
  return useMutation({
    mutationKey: [...taskRunReviewKeys.all(wsId), "create", taskId] as const,
    mutationFn: (input: CreateTaskRunReviewInput) =>
      api.createTaskRunReview(taskId, input),
  });
}
