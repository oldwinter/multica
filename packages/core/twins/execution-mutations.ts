import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { twinKeys } from "./queries";
import { twinExecutionKeys } from "./execution-queries";
import type {
  CreateTwinDepositionInput,
  TwinBriefingPreviewInput,
  TwinFeedbackInput,
  UpsertTwinBindingInput,
} from "./execution-types";

export function useUpsertTwinBinding(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UpsertTwinBindingInput) => api.upsertTwinBinding(input),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.bindings(wsId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.metrics(wsId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.activation(wsId) }),
      ]);
    },
  });
}

export function useDeleteTwinBinding(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (bindingId: string) => api.deleteTwinBinding(bindingId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.bindings(wsId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.metrics(wsId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.activation(wsId) }),
      ]);
    },
  });
}

export function usePreviewTwinBriefing(wsId?: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: TwinBriefingPreviewInput) => api.previewTwinBriefing(input),
    onSuccess: async () => {
      if (wsId) {
        await queryClient.invalidateQueries({ queryKey: twinExecutionKeys.activation(wsId) });
      }
    },
  });
}

export function usePauseTwinExecution(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.pauseTwinExecution(),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.bindings(wsId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.metrics(wsId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.activation(wsId) }),
      ]);
    },
  });
}

export function useSubmitTwinTaskFeedback(wsId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: TwinFeedbackInput) => api.submitTwinTaskFeedback(taskId, input),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.taskContext(wsId, taskId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.metrics(wsId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.activation(wsId) }),
      ]);
    },
  });
}

export function useCreateTwinDeposition(wsId: string, taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input?: CreateTwinDepositionInput) => api.createTwinDeposition(taskId, input),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.taskContext(wsId, taskId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.metrics(wsId) }),
        queryClient.invalidateQueries({ queryKey: twinExecutionKeys.activation(wsId) }),
        queryClient.invalidateQueries({ queryKey: twinKeys.all(wsId) }),
      ]);
    },
  });
}
