// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  RoomCycleSchema,
  RoomMemoryRevisionSchema,
  RoomSchema,
} from "@/data/rooms-schema";
import {
  filterRooms,
  latestRoomCycle,
  latestRoomMemoryRevision,
} from "./room-selectors";

const cycle = (sequence: number) =>
  RoomCycleSchema.parse({
    id: `cycle-${sequence}`,
    sequence,
    source: "manual",
    status: "completed",
    phase: "completed",
  });

const revision = (version: number) =>
  RoomMemoryRevisionSchema.parse({
    id: `revision-${version}`,
    version,
    creator_type: "agent",
    creator_id: "agent-facilitator",
    review_status: "accepted",
  });

const room = (id: string, title: string, decisions: readonly string[] = []) =>
  RoomSchema.parse({
    id,
    workspace_id: "workspace-1",
    title,
    objective: "Choose a rollout",
    status: "active",
    memory: { decisions },
  });

describe("latest Room selectors", () => {
  it.each([
    [[cycle(3), cycle(2), cycle(1)], "cycle-3"],
    [[cycle(1), cycle(2), cycle(3)], "cycle-3"],
  ] as const)("selects the maximum cycle sequence regardless of API order", (items, id) => {
    expect(latestRoomCycle(items)?.id).toBe(id);
  });

  it.each([
    [[revision(3), revision(2), revision(1)], "revision-3"],
    [[revision(1), revision(2), revision(3)], "revision-3"],
  ] as const)("selects the maximum memory version regardless of API order", (items, id) => {
    expect(latestRoomMemoryRevision(items)?.id).toBe(id);
  });
});

describe("filterRooms", () => {
  it("finds Rooms through normalized metadata and accepted memory", () => {
    const active = room("active-room", "Release council");
    const archived = room("archived-room", "Incident archive", [
      "Keep the staged rollback procedure",
    ]);

    expect(filterRooms([active, archived], "STAGED rollback")).toEqual([archived]);
    expect(filterRooms([active, archived], "release rollout")).toEqual([active]);
  });
});
