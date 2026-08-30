import { useMutation, useQueryClient } from "@tanstack/react-query";
import type {
  PostRoomMessageInput,
  PromoteRoomRecommendationInput,
  ReviewRoomSynthesisInput,
  Room,
  RoomDetail,
} from "@/data/rooms-types";
import { api } from "@/data/api";
import { roomKeys } from "@/data/queries/rooms";
import { issueKeys } from "@/data/queries/issue-keys";
import { useWorkspaceStore } from "@/data/workspace-store";

function replaceRoom(
  room: Room,
  detail: RoomDetail | undefined,
): RoomDetail | undefined {
  return detail ? { ...detail, room } : detail;
}

export function usePostRoomMessage(roomId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);

  return useMutation({
    mutationKey: ["postRoomMessage", roomId] as const,
    mutationFn: (input: PostRoomMessageInput) =>
      api.postRoomMessage(roomId, input),
    onSuccess: (result) => {
      if (!result.entry.id) return;
      qc.setQueryData<RoomDetail>(roomKeys.detail(wsId, roomId), (old) => {
        if (!old) return old;
        return {
          ...old,
          entries: [
            ...old.entries.filter((entry) => entry.id !== result.entry.id),
            result.entry,
          ].sort((a, b) => a.ordinal - b.ordinal),
          cycles: [
            ...old.cycles.filter((cycle) => cycle.id !== result.cycle.id),
            result.cycle,
          ].sort((a, b) => a.sequence - b.sequence),
          turns: [
            ...old.turns.filter(
              (turn) => !result.turns.some((incoming) => incoming.id === turn.id),
            ),
            ...result.turns,
          ],
        };
      });
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: roomKeys.detail(wsId, roomId) });
      qc.invalidateQueries({ queryKey: roomKeys.list(wsId) });
      qc.invalidateQueries({ queryKey: roomKeys.preflights(wsId, roomId) });
      qc.invalidateQueries({ queryKey: roomKeys.usage(wsId, roomId) });
    },
  });
}

export function useWakeRoom(roomId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);

  return useMutation({
    mutationKey: ["wakeRoom", roomId] as const,
    mutationFn: (idempotencyKey: string) => api.wakeRoom(roomId, idempotencyKey),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: roomKeys.detail(wsId, roomId) });
      qc.invalidateQueries({ queryKey: roomKeys.list(wsId) });
      qc.invalidateQueries({ queryKey: roomKeys.preflights(wsId, roomId) });
      qc.invalidateQueries({ queryKey: roomKeys.usage(wsId, roomId) });
    },
  });
}

export function useSetRoomStatus(roomId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);

  return useMutation({
    mutationKey: ["setRoomStatus", roomId] as const,
    mutationFn: (status: "active" | "paused" | "archived") =>
      api.setRoomStatus(roomId, status),
    onSuccess: (room) => {
      if (!room.id) return;
      qc.setQueryData<readonly Room[]>(roomKeys.list(wsId), (old) =>
        old?.map((candidate) => (candidate.id === room.id ? room : candidate)),
      );
      qc.setQueryData<RoomDetail>(roomKeys.detail(wsId, roomId), (old) =>
        replaceRoom(room, old),
      );
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: roomKeys.preflights(wsId, roomId) });
    },
  });
}

export function useRetryRoomSynthesis(roomId: string, cycleId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);

  return useMutation({
    mutationKey: ["retryRoomSynthesis", roomId, cycleId] as const,
    mutationFn: (idempotencyKey: string) =>
      api.retryRoomSynthesis(roomId, cycleId, idempotencyKey),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: roomKeys.detail(wsId, roomId) });
      qc.invalidateQueries({ queryKey: roomKeys.usage(wsId, roomId) });
    },
  });
}

export function useReviewRoomSynthesis(roomId: string, cycleId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);

  return useMutation({
    mutationKey: ["reviewRoomSynthesis", roomId, cycleId] as const,
    mutationFn: (input: ReviewRoomSynthesisInput) =>
      api.reviewRoomSynthesis(roomId, cycleId, input),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: roomKeys.detail(wsId, roomId) });
      qc.invalidateQueries({ queryKey: roomKeys.list(wsId) });
      qc.invalidateQueries({ queryKey: roomKeys.preflights(wsId, roomId) });
      qc.invalidateQueries({ queryKey: roomKeys.usage(wsId, roomId) });
    },
  });
}

export function useCancelRoomCycle(roomId: string, cycleId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);

  return useMutation({
    mutationKey: ["cancelRoomCycle", roomId, cycleId] as const,
    mutationFn: (idempotencyKey: string) =>
      api.cancelRoomCycle(roomId, cycleId, idempotencyKey),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: roomKeys.detail(wsId, roomId) });
      qc.invalidateQueries({ queryKey: roomKeys.preflights(wsId, roomId) });
    },
  });
}

export function usePromoteRoomRecommendation(roomId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);

  return useMutation({
    mutationKey: ["promoteRoomRecommendation", roomId] as const,
    mutationFn: (input: PromoteRoomRecommendationInput) =>
      api.promoteRoomRecommendation(roomId, input),
    onSuccess: (artifact) => {
      if (artifact.kind === "issue") {
        qc.invalidateQueries({ queryKey: issueKeys.all(wsId) });
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: roomKeys.detail(wsId, roomId) });
      qc.invalidateQueries({ queryKey: roomKeys.usage(wsId, roomId) });
    },
  });
}

export function useRejectRoomRecommendation(roomId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((state) => state.currentWorkspaceId);

  return useMutation({
    mutationKey: ["rejectRoomRecommendation", roomId] as const,
    mutationFn: (input: {
      memoryRevisionId: string;
      recommendationKey: string;
      idempotencyKey: string;
    }) =>
      api.rejectRoomRecommendation(
        roomId,
        input.memoryRevisionId,
        input.recommendationKey,
        input.idempotencyKey,
      ),
    onSuccess: (review) => {
      if (!review.recommendation_key) return;
      qc.setQueryData<RoomDetail>(roomKeys.detail(wsId, roomId), (old) =>
        old
          ? {
              ...old,
              recommendation_reviews: [
                ...old.recommendation_reviews.filter(
                  (candidate) =>
                    candidate.memory_revision_id !== review.memory_revision_id ||
                    candidate.recommendation_key !== review.recommendation_key,
                ),
                review,
              ],
            }
          : old,
      );
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: roomKeys.detail(wsId, roomId) });
    },
  });
}
