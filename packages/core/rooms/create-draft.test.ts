import { describe, expect, it } from "vitest";
import {
  applyRoomTemplateDefaults,
  duplicateRoomConfiguration,
  rankRoomsForValueReview,
  type RoomTemplateDraftFields,
} from "./create-draft";
import { EMPTY_ROOM_DETAIL } from "./schemas";
import type { Room, RoomDetail } from "./types";

const defaults = (objective: string): RoomTemplateDraftFields => ({
  objective,
  successCriteria: `${objective} criteria`,
  stopConditions: `${objective} stop`,
  instructions: `${objective} instructions`,
  dailyTurnLimit: "8",
  maxCostTicks: "1200",
  scheduleMinutes: "60",
});

describe("applyRoomTemplateDefaults", () => {
  it("updates derived defaults while preserving every user-edited field", () => {
    expect(
      applyRoomTemplateDefaults(defaults("research"), defaults("decision"), {
        objective: true,
        stopConditions: true,
        maxCostTicks: true,
      }),
    ).toEqual({
      objective: "research",
      successCriteria: "decision criteria",
      stopConditions: "research stop",
      instructions: "decision instructions",
      dailyTurnLimit: "8",
      maxCostTicks: "1200",
      scheduleMinutes: "60",
    });
  });
});

describe("duplicateRoomConfiguration", () => {
  it("copies configuration only and pauses a duplicated schedule", () => {
    const detail: RoomDetail = {
      ...EMPTY_ROOM_DETAIL,
      room: {
        ...EMPTY_ROOM_DETAIL.room,
        id: "room-source",
        title: "Weekly risk review",
        objective: "Find launch risks",
        instructions: "Preserve dissent",
        success_criteria: ["Risks are cited"],
        stop_conditions: ["Owner accepts"],
        template_id: "risk",
        facilitator_agent_id: "agent-lead",
        facilitator_squad_id: "squad-risk",
        daily_turn_limit: 8,
        max_cost_ticks: 1200,
        schedule_interval_minutes: 1440,
        memory_version: 7,
        accepted_memory_revision_id: "revision-accepted",
      },
      participants: [
        {
          id: "participant-facilitator",
          type: "agent",
          participant_id: "agent-lead",
          role: "facilitator",
          source_squad_id: "squad-risk",
          joined_at: "2026-08-01T00:00:00Z",
        },
        {
          id: "participant-squad",
          type: "agent",
          participant_id: "agent-squad",
          role: "participant",
          source_squad_id: "squad-risk",
          joined_at: "2026-08-01T00:00:00Z",
        },
        {
          id: "participant-explicit",
          type: "member",
          participant_id: "member-reviewer",
          role: "observer",
          source_squad_id: null,
          joined_at: "2026-08-01T00:00:00Z",
        },
      ],
      entries: [{ id: "entry-history" }] as unknown as RoomDetail["entries"],
      cycles: [{ id: "cycle-history" }] as unknown as RoomDetail["cycles"],
      memory_revisions: [
        { id: "revision-history" },
      ] as unknown as RoomDetail["memory_revisions"],
      artifacts: [
        { id: "artifact-history" },
      ] as unknown as RoomDetail["artifacts"],
    };

    const duplicate = duplicateRoomConfiguration(detail);

    expect(duplicate).toEqual({
      title: "Weekly risk review",
      instructions: "Preserve dissent",
      objective: "Find launch risks",
      success_criteria: ["Risks are cited"],
      stop_conditions: ["Owner accepts"],
      template_id: "risk",
      participants: [
        { type: "member", id: "member-reviewer", role: "observer" },
      ],
      daily_turn_limit: 8,
      max_cost_ticks: 1200,
      schedule_interval_minutes: 1440,
      start_paused: true,
      facilitator_squad_id: "squad-risk",
    });
    expect(duplicate).not.toHaveProperty("id");
    expect(duplicate).not.toHaveProperty("memory");
    expect(duplicate).not.toHaveProperty("cycles");
    expect(duplicate).not.toHaveProperty("idempotency_key");
  });
});

describe("rankRoomsForValueReview", () => {
  it("ranks only recurring or recently active Rooms by accepted value", () => {
    const now = Date.parse("2026-08-26T00:00:00Z");
    const room = (
      id: string,
      accepted: number,
      repeat: number,
      lastRunAt: string | null,
    ): Room => ({
      ...EMPTY_ROOM_DETAIL.room,
      id,
      title: id,
      value: {
        last_accepted_revision_id: null,
        last_accepted_at: null,
        last_cycle_id: null,
        last_run_status: "completed",
        last_run_phase: "completed",
        last_run_reason: null,
        last_run_at: lastRunAt,
        last_run_cost_ticks: 0,
        repeat_run_count: repeat,
        accepted_outcomes: accepted,
        active_weeks: 1,
        accepted_outcomes_per_active_week: accepted,
        median_review_latency_seconds: 0,
        promotion_rate: accepted > 0 ? 1 : 0,
        failed_cycles: 0,
        refused_cycles: 0,
      },
    });

    expect(
      rankRoomsForValueReview(
        [
          room("stale-single", 9, 0, "2026-06-01T00:00:00Z"),
          room("recent", 1, 0, "2026-08-20T00:00:00Z"),
          room("recurring", 3, 2, "2026-06-01T00:00:00Z"),
        ],
        now,
      ).map((entry) => entry.id),
    ).toEqual(["recurring", "recent"]);
  });
});
