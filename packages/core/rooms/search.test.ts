// @vitest-environment node

import { describe, expect, it } from "vitest";
import { EMPTY_ROOM } from "./schemas";
import { filterRooms } from "./search";
import type { Room } from "./types";

const room = (overrides: Partial<Room>): Room => ({
  ...EMPTY_ROOM,
  id: "room-1",
  title: "Release council",
  objective: "Choose a rollout",
  status: "active",
  ...overrides,
});

describe("filterRooms", () => {
  it("finds archived Rooms through accepted decision memory", () => {
    const archived = room({
      id: "archived-room",
      title: "Incident archive",
      status: "archived",
      memory: {
        ...EMPTY_ROOM.memory,
        decisions: ["Keep the staged rollback procedure"],
      },
    });
    const active = room({ id: "active-room" });

    expect(filterRooms([active, archived], "staged rollback")).toEqual([archived]);
  });

  it("matches every normalized term across Room metadata", () => {
    expect(filterRooms([room({})], "RELEASE rollout")).toHaveLength(1);
    expect(filterRooms([room({})], "release missing")).toHaveLength(0);
  });
});
