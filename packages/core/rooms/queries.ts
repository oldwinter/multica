import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const roomKeys = {
  all: (wsId: string) => ["rooms", wsId] as const,
  list: (wsId: string) => [...roomKeys.all(wsId), "list"] as const,
  detail: (wsId: string, roomId: string) =>
    [...roomKeys.all(wsId), "detail", roomId] as const,
};

export function roomListOptions(wsId: string) {
  return queryOptions({
    queryKey: roomKeys.list(wsId),
    queryFn: () => api.listRooms(),
    enabled: Boolean(wsId),
  });
}

export function roomDetailOptions(wsId: string, roomId: string) {
  return queryOptions({
    queryKey: roomKeys.detail(wsId, roomId),
    queryFn: () => api.getRoom(roomId),
    enabled: Boolean(wsId && roomId),
  });
}
