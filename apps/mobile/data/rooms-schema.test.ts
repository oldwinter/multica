// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  RoomCycleSchema,
  RoomDetailSchema,
  RoomPreflightSchema,
  RoomSchema,
  RoomUsageSchema,
} from "./rooms-schema";

describe("Room mobile wire schemas", () => {
  it("defaults additive Room fields while preserving identity", () => {
    const room = RoomSchema.parse({
      id: "room-1",
      workspace_id: "ws-1",
      title: "Planning",
      status: "active",
    });

    expect(room.objective).toBe("");
    expect(room.success_criteria).toEqual([]);
    expect(room.max_cost_ticks).toBeNull();
    expect(room.accepted_memory_revision_id).toBeNull();
		expect(room.value).toBeNull();
  });

  it("preserves Room value signals and additive usage metrics", () => {
    const room = RoomSchema.parse({
      id: "room-value",
      workspace_id: "ws-1",
      status: "active",
      value: {
        last_run_status: "completed",
        last_run_phase: "completed",
        last_run_cost_ticks: 18,
        repeat_run_count: 2,
        accepted_outcomes: 3,
        promotion_rate: 0.5,
      },
    });
    const usage = RoomUsageSchema.parse({
      repeat_run_count: 2,
      active_weeks: 2,
      accepted_outcomes_per_active_week: 1.5,
      median_review_latency_seconds: 90,
      promotion_rate: 0.5,
      failed_cycles: 1,
      refused_cycles: 2,
      cost_ticks_per_accepted_outcome: 6,
    });

    expect(room.value).toMatchObject({
      last_run_status: "completed",
      last_run_cost_ticks: 18,
      repeat_run_count: 2,
      accepted_outcomes: 3,
      promotion_rate: 0.5,
    });
    expect(usage).toMatchObject({
      accepted_outcomes_per_active_week: 1.5,
      median_review_latency_seconds: 90,
      cost_ticks_per_accepted_outcome: 6,
    });
  });

  it("renders future enum values as unknown instead of dropping detail", () => {
    const detail = RoomDetailSchema.parse({
      room: {
        id: "room-1",
        workspace_id: "ws-1",
        title: "Planning",
        status: "future_status",
      },
      cycles: [
        {
          id: "cycle-1",
          sequence: 1,
          source: "future_source",
          status: "future_status",
          phase: "future_phase",
        },
      ],
    });

    expect(detail.room.status).toBe("unknown");
    expect(detail.cycles[0]?.source).toBe("unknown");
    expect(detail.cycles[0]?.phase).toBe("unknown");
  });

  it("keeps structured synthesis errors and retry attempts auditable", () => {
    const cycle = RoomCycleSchema.parse({
      id: "cycle-1",
      sequence: 1,
      source: "manual",
      status: "failed",
      phase: "failed",
      synthesis_error: {
        code: "invalid_output",
        message: "Missing citations",
        retryable: true,
      },
    });
    const detail = RoomDetailSchema.parse({
      room: { id: "room-1", workspace_id: "ws-1", status: "active" },
      cycles: [cycle],
      turns: [
        {
          id: "turn-1",
          cycle_id: "cycle-1",
          turn_kind: "synthesis",
          status: "failed",
          attempt: 2,
        },
      ],
    });

    expect(detail.cycles[0]?.synthesis_error?.retryable).toBe(true);
    expect(detail.turns[0]?.attempt).toBe(2);
  });

  it("keeps daemon capability blockers actionable", () => {
    const preflight = RoomPreflightSchema.parse({
      capability_version: 2,
      capability_ready: false,
      required_daemon_capability: "room_outcomes_v2",
      target_agents: [
        {
          agent_id: "agent-1",
          reason: "daemon_capability_unavailable",
        },
      ],
    });

    expect(preflight.capability_version).toBe(2);
    expect(preflight.capability_ready).toBe(false);
    expect(preflight.required_daemon_capability).toBe("room_outcomes_v2");
    expect(preflight.target_agents[0]?.reason).toBe(
      "daemon_capability_unavailable",
    );
  });

	it("keeps cost reservations and unsupported execution actionable", () => {
		const cycle = RoomCycleSchema.parse({
			id: "cycle-cost",
			status: "queued",
			phase: "gathering",
			source: "manual",
			cost_limit_ticks: 42,
		});
		const preflight = RoomPreflightSchema.parse({
			spend_limit_supported: false,
			required_cost_capability: "room-cost-limits-v1",
			refusal_reason: "spend_limit_unsupported",
			budget: { reserved_cost_ticks: 42 },
		});

		expect(cycle.cost_limit_ticks).toBe(42);
		expect(preflight.spend_limit_supported).toBe(false);
		expect(preflight.required_cost_capability).toBe("room-cost-limits-v1");
		expect(preflight.budget.reserved_cost_ticks).toBe(42);
	});
});
