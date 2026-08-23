// @vitest-environment node

import { readFileSync } from "node:fs";
import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
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
import {
  applyRoomArtifactEvent,
  applyRoomCreatedEvent,
  applyRoomCycleEvent,
  applyRoomEntryEvent,
  applyRoomMemoryRevisionEvent,
  applyRoomRecommendationReviewEvent,
  applyRoomReviewEvent,
  applyRoomTurnEvent,
  applyRoomUpdatedEvent,
} from "./realtime";

const workspaceId = "workspace-1";
const roomId = "00000000-0000-0000-0000-000000000001";
const fixtures = JSON.parse(readFileSync(
  new URL("../../../server/internal/room/testdata/realtime_events.json", import.meta.url),
  "utf8",
)) as Record<string, unknown>;

const contracts = [
  ["room:created", RoomCreatedPayloadSchema, applyRoomCreatedEvent],
  ["room:updated", RoomUpdatedPayloadSchema, applyRoomUpdatedEvent],
  ["room:entry", RoomEntryPayloadSchema, applyRoomEntryEvent],
  ["room:cycle", RoomCyclePayloadSchema, applyRoomCycleEvent],
  ["room:turn", RoomTurnPayloadSchema, applyRoomTurnEvent],
  ["room:memory_revision", RoomMemoryRevisionPayloadSchema, applyRoomMemoryRevisionEvent],
  ["room:review", RoomReviewPayloadSchema, applyRoomReviewEvent],
  [
    "room:recommendation_review",
    RoomRecommendationReviewPayloadSchema,
    applyRoomRecommendationReviewEvent,
  ],
  ["room:artifact", RoomArtifactPayloadSchema, applyRoomArtifactEvent],
] as const;

describe("Room realtime wire contract", () => {
  it.each(contracts)("parses and targets the server %s fixture", (name, schema, apply) => {
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    expect(schema.safeParse(fixtures[name]).success).toBe(true);
    expect(apply(queryClient, workspaceId, fixtures[name])).toBe("invalidated");
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: name === "room:created"
        ? roomKeys.list(workspaceId)
        : roomKeys.detail(workspaceId, roomId),
    });
  });

  it("invalidates the Room tree when a future payload has no usable identity", () => {
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    expect(applyRoomEntryEvent(queryClient, workspaceId, { entry_id: "entry-1" }))
      .toBe("invalidated");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: roomKeys.all(workspaceId) });
  });

  it("rejects nested DTOs that contradict the bounded signal contract", () => {
    expect(RoomCreatedPayloadSchema.safeParse({ room: { id: roomId } }).success).toBe(false);
    expect(RoomEntryPayloadSchema.safeParse({
      ...fixtures["room:entry"] as object,
      entry: { id: "private-body" },
    }).success).toBe(false);
  });
});
