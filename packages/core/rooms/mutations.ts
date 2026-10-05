import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { issueKeys } from "../issues/queries";
import { wikiKeys } from "../wiki/queries";
import { roomKeys } from "./queries";
import type {
  CreateRoomInput,
  CancelRoomCycleInput,
  PostRoomMessageInput,
  PromoteRoomArtifactInput,
  RejectRoomRecommendationInput,
  RetryRoomSynthesisInput,
  ReviewRoomCycleInput,
  SetRoomStatusInput,
  UpdateRoomBudgetInput,
  WakeRoomInput,
} from "./types";

export function useCreateRoom() {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (input: CreateRoomInput) => api.createRoom(input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}

export function usePostRoomMessage(roomId: string) {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (input: PostRoomMessageInput) => api.postRoomMessage(roomId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}

export function useWakeRoom(roomId: string) {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (input: WakeRoomInput) => api.wakeRoom(roomId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}

export function useSetRoomStatus(roomId: string) {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (input: SetRoomStatusInput) => api.setRoomStatus(roomId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}

export function useUpdateRoomBudget(roomId: string) {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (input: UpdateRoomBudgetInput) => api.updateRoomBudget(roomId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}

export function usePromoteRoomArtifact(roomId: string) {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (input: PromoteRoomArtifactInput) => api.promoteRoomArtifact(roomId, input),
    onSuccess: (artifact) => {
      if (artifact.kind === "issue") {
        queryClient.invalidateQueries({ queryKey: issueKeys.all(wsId) });
      }
      if (artifact.kind === "wiki") {
        queryClient.invalidateQueries({ queryKey: wikiKeys.all(wsId) });
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}

export function useRetryRoomSynthesis(roomId: string, cycleId: string) {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (input: RetryRoomSynthesisInput) =>
      api.retryRoomSynthesis(roomId, cycleId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}

export function useReviewRoomCycle(roomId: string, cycleId: string) {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (input: ReviewRoomCycleInput) =>
      api.reviewRoomCycle(roomId, cycleId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}

export function useCancelRoomCycle(roomId: string, cycleId: string) {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (input: CancelRoomCycleInput) =>
      api.cancelRoomCycle(roomId, cycleId, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}

export function useReviewRoomRecommendation(
  roomId: string,
) {
  const queryClient = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({
      revisionId,
      recommendationKey,
      input,
    }: {
      readonly revisionId: string;
      readonly recommendationKey: string;
      readonly input: RejectRoomRecommendationInput;
    }) => api.reviewRoomRecommendation(roomId, revisionId, recommendationKey, input),
    onSettled: () => queryClient.invalidateQueries({ queryKey: roomKeys.all(wsId) }),
  });
}
