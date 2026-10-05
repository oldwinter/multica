import type { QueryClient } from "@tanstack/react-query";
import type { z } from "zod";
import type { WSClient } from "../api/ws-client";
import {
  RoomArtifactPayloadSchema,
  RoomCreatedPayloadSchema,
  RoomCyclePayloadSchema,
  RoomEntryPayloadSchema,
  RoomMemoryRevisionPayloadSchema,
  RoomRecommendationReviewPayloadSchema,
  RoomReviewPayloadSchema,
  RoomTurnPayloadSchema,
  RoomUpdatedPayloadSchema,
} from "./event-schemas";
import { roomKeys } from "./queries";

export type RoomRealtimeUpdateResult = "invalidated" | "ignored";

type RoomSignalSchema = z.ZodType<{ readonly room_id: string }>;

function roomSignal(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
  schema: RoomSignalSchema,
  options: { readonly list?: boolean; readonly outcome?: boolean } = {},
): RoomRealtimeUpdateResult {
  if (!workspaceId) return "ignored";
  const parsed = schema.safeParse(payload);
  const roomId = parsed.success ? parsed.data.room_id : directString(payload, "room_id");

  if (options.list) {
    void queryClient.invalidateQueries({ queryKey: roomKeys.list(workspaceId) });
  }
  void queryClient.invalidateQueries({
    queryKey: roomId ? roomKeys.detail(workspaceId, roomId) : roomKeys.all(workspaceId),
  });
  if (options.outcome && roomId) {
    void queryClient.invalidateQueries({ queryKey: roomKeys.preflights(workspaceId, roomId) });
    void queryClient.invalidateQueries({ queryKey: roomKeys.usage(workspaceId, roomId) });
  }
  return "invalidated";
}

export function applyRoomCreatedEvent(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
): RoomRealtimeUpdateResult {
  return roomSignal(queryClient, workspaceId, payload, RoomCreatedPayloadSchema, { list: true });
}

export function applyRoomUpdatedEvent(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
): RoomRealtimeUpdateResult {
  return roomSignal(queryClient, workspaceId, payload, RoomUpdatedPayloadSchema, {
    list: true,
    outcome: true,
  });
}

export function applyRoomEntryEvent(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
): RoomRealtimeUpdateResult {
  return roomSignal(queryClient, workspaceId, payload, RoomEntryPayloadSchema);
}

export function applyRoomCycleEvent(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
): RoomRealtimeUpdateResult {
  return roomSignal(queryClient, workspaceId, payload, RoomCyclePayloadSchema, {
    list: true,
    outcome: true,
  });
}

export function applyRoomTurnEvent(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
): RoomRealtimeUpdateResult {
  return roomSignal(queryClient, workspaceId, payload, RoomTurnPayloadSchema, {
    list: true,
    outcome: true,
  });
}

export function applyRoomMemoryRevisionEvent(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
): RoomRealtimeUpdateResult {
  return roomSignal(queryClient, workspaceId, payload, RoomMemoryRevisionPayloadSchema, {
    list: true,
    outcome: true,
  });
}

export function applyRoomReviewEvent(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
): RoomRealtimeUpdateResult {
  return roomSignal(queryClient, workspaceId, payload, RoomReviewPayloadSchema, {
    list: true,
    outcome: true,
  });
}

export function applyRoomRecommendationReviewEvent(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
): RoomRealtimeUpdateResult {
  return roomSignal(queryClient, workspaceId, payload, RoomRecommendationReviewPayloadSchema);
}

export function applyRoomArtifactEvent(
  queryClient: QueryClient,
  workspaceId: string | null | undefined,
  payload: unknown,
): RoomRealtimeUpdateResult {
  return roomSignal(queryClient, workspaceId, payload, RoomArtifactPayloadSchema, { list: true });
}

function directString(payload: unknown, key: string): string | null {
  if (!payload || typeof payload !== "object") return null;
  const value = (payload as Record<string, unknown>)[key];
  return typeof value === "string" && value.length > 0 ? value : null;
}

/** One registration boundary for the complete Rooms realtime contract. */
export function subscribeRoomRealtime(
  ws: WSClient,
  queryClient: QueryClient,
  getWorkspaceId: () => string | null | undefined,
): () => void {
  const unsubscribers = [
    ws.on("room:created", (payload) => {
      applyRoomCreatedEvent(queryClient, getWorkspaceId(), payload);
    }),
    ws.on("room:updated", (payload) => {
      applyRoomUpdatedEvent(queryClient, getWorkspaceId(), payload);
    }),
    ws.on("room:entry", (payload) => {
      applyRoomEntryEvent(queryClient, getWorkspaceId(), payload);
    }),
    ws.on("room:cycle", (payload) => {
      applyRoomCycleEvent(queryClient, getWorkspaceId(), payload);
    }),
    ws.on("room:turn", (payload) => {
      applyRoomTurnEvent(queryClient, getWorkspaceId(), payload);
    }),
    ws.on("room:memory_revision", (payload) => {
      applyRoomMemoryRevisionEvent(queryClient, getWorkspaceId(), payload);
    }),
    ws.on("room:review", (payload) => {
      applyRoomReviewEvent(queryClient, getWorkspaceId(), payload);
    }),
    ws.on("room:recommendation_review", (payload) => {
      applyRoomRecommendationReviewEvent(queryClient, getWorkspaceId(), payload);
    }),
    ws.on("room:artifact", (payload) => {
      applyRoomArtifactEvent(queryClient, getWorkspaceId(), payload);
    }),
  ];
  return () => {
    for (const unsubscribe of unsubscribers) unsubscribe();
  };
}
