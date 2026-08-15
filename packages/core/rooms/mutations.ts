import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { issueKeys } from "../issues/queries";
import { wikiKeys } from "../wiki/queries";
import { roomKeys } from "./queries";
import type {
  CreateRoomInput,
  PostRoomMessageInput,
  PromoteRoomArtifactInput,
  SetRoomStatusInput,
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
