import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";

export const roomKeys = {
  all: (wsId: string) => ["rooms", wsId] as const,
  list: (wsId: string) => [...roomKeys.all(wsId), "list"] as const,
  detail: (wsId: string, roomId: string) =>
    [...roomKeys.all(wsId), "detail", roomId] as const,
  preflights: (wsId: string, roomId: string) =>
    [...roomKeys.detail(wsId, roomId), "preflight"] as const,
  preflight: (
    wsId: string,
    roomId: string,
    targetAgentId?: string,
    source: "manual" | "schedule" = "manual",
  ) => [...roomKeys.preflights(wsId, roomId), source, targetAgentId ?? "all"] as const,
  usage: (wsId: string, roomId: string) =>
    [...roomKeys.detail(wsId, roomId), "usage"] as const,
};

export function roomListOptions(wsId: string) {
  return queryOptions({
    queryKey: roomKeys.list(wsId),
    queryFn: () => api.listRooms(),
    enabled: Boolean(wsId),
  });
}

export function roomPreflightOptions(
  wsId: string,
  roomId: string,
  targetAgentId?: string,
  source: "manual" | "schedule" = "manual",
) {
  return queryOptions({
    queryKey: roomKeys.preflight(wsId, roomId, targetAgentId, source),
    queryFn: () => api.getRoomPreflight(roomId, targetAgentId, source),
    enabled: Boolean(wsId && roomId),
    staleTime: 10_000,
  });
}

export function roomUsageOptions(wsId: string, roomId: string) {
  return queryOptions({
    queryKey: roomKeys.usage(wsId, roomId),
    queryFn: () => api.getRoomUsage(roomId),
    enabled: Boolean(wsId && roomId),
    staleTime: 15_000,
  });
}

export function roomDetailOptions(wsId: string, roomId: string) {
  return queryOptions({
    queryKey: roomKeys.detail(wsId, roomId),
    queryFn: () => api.getRoom(roomId),
    enabled: Boolean(wsId && roomId),
  });
}
