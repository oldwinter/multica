import { queryOptions } from "@tanstack/react-query";
import { api } from "@/data/api";

export const roomKeys = {
  all: (wsId: string | null) => ["rooms", wsId] as const,
  list: (wsId: string | null) => [...roomKeys.all(wsId), "list"] as const,
  detail: (wsId: string | null, roomId: string) =>
    [...roomKeys.all(wsId), "detail", roomId] as const,
  preflights: (wsId: string | null, roomId: string) =>
    [...roomKeys.all(wsId), "preflight", roomId] as const,
  preflight: (
    wsId: string | null,
    roomId: string,
    source: "manual" | "schedule" = "manual",
  ) => [...roomKeys.preflights(wsId, roomId), source] as const,
  usage: (wsId: string | null, roomId: string) =>
    [...roomKeys.all(wsId), "usage", roomId] as const,
};

export const roomListOptions = (wsId: string | null) =>
  queryOptions({
    queryKey: roomKeys.list(wsId),
    queryFn: ({ signal }) => api.listRooms({ signal }),
    enabled: !!wsId,
  });

export const roomDetailOptions = (wsId: string | null, roomId: string) =>
  queryOptions({
    queryKey: roomKeys.detail(wsId, roomId),
    queryFn: ({ signal }) => api.getRoom(roomId, { signal }),
    enabled: !!wsId && !!roomId,
  });

export const roomPreflightOptions = (
  wsId: string | null,
  roomId: string,
  source: "manual" | "schedule" = "manual",
) =>
  queryOptions({
    queryKey: roomKeys.preflight(wsId, roomId, source),
    queryFn: ({ signal }) => api.getRoomPreflight(roomId, { signal, source }),
    enabled: !!wsId && !!roomId,
    staleTime: 15_000,
  });

export const roomUsageOptions = (wsId: string | null, roomId: string) =>
  queryOptions({
    queryKey: roomKeys.usage(wsId, roomId),
    queryFn: ({ signal }) => api.getRoomUsage(roomId, { signal }),
    enabled: !!wsId && !!roomId,
    staleTime: 30_000,
  });
