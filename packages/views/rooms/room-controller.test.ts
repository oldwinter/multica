// @vitest-environment node

import { describe, expect, it } from "vitest";
import { EMPTY_ROOM_DETAIL, type RoomCycle } from "@multica/core/rooms";
import { selectRecentRoomCycles, selectRoomLifecycleCycleId } from "./room-controller";

const roomCycle = (sequence: number): RoomCycle => ({
  id: `cycle-${sequence}`,
  sequence,
  source: "manual",
  wake_key: `wake-${sequence}`,
  triggering_entry_id: null,
  status: "completed",
  phase: "completed",
  refusal_reason: null,
  synthesis_error: null,
  synthesis_turn_id: null,
  memory_revision_id: null,
  expected_max_turns: 2,
	cost_limit_ticks: null,
  planned_at: null,
  created_at: `2026-08-23T00:0${sequence}:00Z`,
  started_at: null,
  completed_at: `2026-08-23T00:0${sequence}:30Z`,
});

describe("selectRoomLifecycleCycleId", () => {
  it.each([
    ["descending", [3, 2, 1]],
    ["ascending", [1, 2, 3]],
  ] as const)("targets review and retry at the newest cycle from an %s API response", (_, order) => {
    const detail = {
      ...EMPTY_ROOM_DETAIL,
      room: { ...EMPTY_ROOM_DETAIL.room, active_cycle_id: null },
      cycles: order.map(roomCycle),
    };

    expect(selectRoomLifecycleCycleId(detail)).toBe("cycle-3");
  });

  it("prefers the authoritative active cycle pointer", () => {
    const detail = {
      ...EMPTY_ROOM_DETAIL,
      room: { ...EMPTY_ROOM_DETAIL.room, active_cycle_id: "cycle-2" },
      cycles: [roomCycle(3), {
        ...roomCycle(2),
        status: "running" as const,
        phase: "awaiting_review" as const,
      }],
    };

    expect(selectRoomLifecycleCycleId(detail)).toBe("cycle-2");
  });

  it("ignores a stale terminal active pointer", () => {
    const detail = {
      ...EMPTY_ROOM_DETAIL,
      room: { ...EMPTY_ROOM_DETAIL.room, active_cycle_id: "cycle-1" },
      cycles: [
        { ...roomCycle(3), status: "running" as const, phase: "synthesizing" as const },
        roomCycle(1),
      ],
    };

    expect(selectRoomLifecycleCycleId(detail)).toBe("cycle-3");
  });

  it("shows the newest six cycles without mutating a descending response", () => {
    const cycles = [7, 6, 5, 4, 3, 2, 1].map(roomCycle);
    const selected = selectRecentRoomCycles(cycles);

    expect(selected.map((cycle) => cycle.sequence)).toEqual([7, 6, 5, 4, 3, 2]);
    expect(cycles.map((cycle) => cycle.sequence)).toEqual([7, 6, 5, 4, 3, 2, 1]);
  });
});
