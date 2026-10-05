// @vitest-environment node

import { describe, expect, it } from "vitest";
import { EMPTY_ROOM_DETAIL } from "./schemas";
import { deriveMemoryDiff, deriveRoomOutcomeState, selectLatestRoomCycle } from "./outcome-state";
import type {
  RoomCycle,
  RoomDetail,
  RoomMemoryRevision,
  RoomPreflight,
  RoomSynthesis,
} from "./types";

const synthesis = (summary: string, facts: readonly string[] = []): RoomSynthesis => ({
  schema_version: 1,
  summary,
  facts: facts.map((text, index) => ({
    text,
    citation_entry_ids: [`entry-${index}`],
    confidence: 0.8,
  })),
  decisions: [],
  open_questions: [],
  disagreements: [],
  action_items: [],
  recommendations: [],
  confidence: 0.8,
});

const cycle = (overrides: Partial<RoomCycle> = {}): RoomCycle => ({
  id: "cycle-1",
  sequence: 1,
  source: "manual",
  wake_key: "wake-1",
  triggering_entry_id: null,
  status: "running",
  phase: "gathering",
  refusal_reason: null,
  synthesis_error: null,
  synthesis_turn_id: null,
  memory_revision_id: null,
  expected_max_turns: 3,
	cost_limit_ticks: null,
  planned_at: null,
  created_at: "2026-08-23T00:00:00Z",
  started_at: null,
  completed_at: null,
  ...overrides,
});

const revision = (
  id: string,
  version: number,
  reviewStatus: RoomMemoryRevision["review_status"],
  value: RoomSynthesis,
): RoomMemoryRevision => ({
  id,
  room_id: "room-1",
  cycle_id: "cycle-1",
  synthesis_turn_id: "turn-synthesis",
  version,
  schema_version: 1,
  synthesis: value,
  digest: `digest-${version}`,
  creator_type: "agent",
  creator_id: "agent-facilitator",
  review_status: reviewStatus,
  reviewed_by_user_id: null,
  reviewed_at: null,
  corrected_from_revision_id: null,
  created_at: `2026-08-23T00:0${version}:00Z`,
});

const detail = (overrides: Partial<RoomDetail> = {}): RoomDetail => ({
  ...EMPTY_ROOM_DETAIL,
  room: {
    ...EMPTY_ROOM_DETAIL.room,
    id: "room-1",
    status: "active",
    objective: "Choose a release strategy",
  },
  ...overrides,
});

const allowedPreflight: RoomPreflight = {
  source: "manual",
  allowed: true,
  refusal_reason: null,
  capability_version: 1,
  capability_ready: true,
  required_daemon_capability: "rooms_outcome_v1",
	spend_limit_supported: true,
	required_cost_capability: "",
  target_agents: [],
  expected_max_turns: 3,
  synthesis_required: true,
  budget: {
    daily_turn_limit: 10,
    used_turns: 2,
    max_cost_ticks: 100,
    used_cost_ticks: 20,
    remaining_cost_ticks: 80,
		reserved_cost_ticks: 0,
    uncosted_turns: 0,
  },
};

describe("deriveRoomOutcomeState", () => {
  it("uses authoritative preflight before offering a run", () => {
    expect(deriveRoomOutcomeState(detail()).nextAction).toBe("run_preflight");
    expect(
      deriveRoomOutcomeState(detail(), { preflight: allowedPreflight }).nextAction,
    ).toBe("run_cycle");
    expect(
      deriveRoomOutcomeState(detail(), {
        preflight: { ...allowedPreflight, allowed: false, refusal_reason: "budget_exhausted" },
      }),
    ).toMatchObject({ nextAction: "resolve_blocker", blocker: "budget_exhausted" });
  });

  it("surfaces an actionable daemon capability blocker", () => {
    expect(deriveRoomOutcomeState(detail(), {
      preflight: {
        ...allowedPreflight,
        allowed: false,
        capability_ready: false,
        required_daemon_capability: "rooms_outcome_v2",
      },
    })).toMatchObject({
      nextAction: "resolve_blocker",
      blocker: "daemon_capability_unavailable",
    });
  });

  it("maps active phases to wait/cancel and pending synthesis to review", () => {
    const gathering = detail({
      room: { ...detail().room, active_cycle_id: "cycle-1" },
      cycles: [cycle()],
    });
    expect(deriveRoomOutcomeState(gathering)).toMatchObject({
      phase: "gathering",
      nextAction: "wait",
      canCancel: true,
    });

    const pending = revision("revision-2", 2, "pending", synthesis("New outcome"));
    const review = detail({
      cycles: [cycle({ status: "completed", phase: "awaiting_review", memory_revision_id: pending.id })],
      memory_revisions: [pending],
    });
    expect(deriveRoomOutcomeState(review)).toMatchObject({
      phase: "awaiting_review",
      nextAction: "review",
      latestOutcome: pending,
    });
  });

  it("ignores a stale terminal active pointer after a newer cycle event", () => {
    const state = deriveRoomOutcomeState(detail({
      room: { ...detail().room, active_cycle_id: "cycle-1" },
      cycles: [
        cycle({ id: "cycle-2", sequence: 2 }),
        cycle({ id: "cycle-1", status: "completed", phase: "completed" }),
      ],
    }));

    expect(state.activeCycle?.id).toBe("cycle-2");
    expect(state.canCancel).toBe(true);
  });

  it("offers synthesis-only retry without rerunning participants", () => {
    const failed = detail({
      cycles: [cycle({
        status: "failed",
        phase: "failed",
        synthesis_error: { code: "malformed_synthesis", message: "Invalid JSON", retryable: true },
      })],
    });
    expect(deriveRoomOutcomeState(failed)).toMatchObject({
      nextAction: "retry_synthesis",
      blocker: "malformed_synthesis",
      canCancel: false,
    });
  });

  it.each([
    ["ascending", [1, 2, 3]],
    ["descending", [3, 2, 1]],
  ] as const)("selects the highest cycle and revision from %s API order", (_, order) => {
    const cycles = order.map((sequence) => cycle({
      id: `cycle-${sequence}`,
      sequence,
      status: "completed",
      phase: "completed",
    }));
    const revisions = order.map((version) =>
      revision(`revision-${version}`, version, version === 3 ? "pending" : "accepted", synthesis(`Outcome ${version}`)),
    );

    const state = deriveRoomOutcomeState(detail({ cycles, memory_revisions: revisions }));
    expect(selectLatestRoomCycle(cycles)?.sequence).toBe(3);
    expect(state.latestCycle?.sequence).toBe(3);
    expect(state.latestOutcome?.version).toBe(3);
    expect(state.acceptedOutcome?.version).toBe(2);
  });

  it("keeps accepted and candidate revisions separate after a correction", () => {
    const accepted = revision("revision-1", 1, "accepted", synthesis("First", ["A", "B"]));
    const corrected = { ...accepted, review_status: "corrected" as const };
    const candidate = {
      ...revision("revision-2", 2, "pending", synthesis("Second", ["B", "C"])),
      corrected_from_revision_id: accepted.id,
    };
    const state = deriveRoomOutcomeState(detail({
      room: { ...detail().room, accepted_memory_revision_id: accepted.id },
      memory_revisions: [corrected, candidate],
      cycles: [cycle({ status: "completed", phase: "awaiting_review" })],
    }));
    expect(state.acceptedOutcome?.id).toBe(accepted.id);
    expect(state.memoryDiff).toEqual({ summaryChanged: true, added: ["C"], removed: ["A"] });
  });

  it("keeps the latest change visible after the new revision is accepted", () => {
    const previous = revision(
      "revision-1",
      1,
      "corrected",
      synthesis("Initial outcome", ["Old fact"]),
    );
    const accepted = revision(
      "revision-2",
      2,
      "accepted",
      synthesis("Accepted outcome", ["New fact"]),
    );
    const state = deriveRoomOutcomeState(detail({
      room: {
        ...detail().room,
        accepted_memory_revision_id: accepted.id,
      },
      memory_revisions: [accepted, previous],
    }));

    expect(state.acceptedOutcome?.id).toBe(accepted.id);
    expect(state.memoryDiff).toEqual({
      summaryChanged: true,
      added: ["New fact"],
      removed: ["Old fact"],
    });
  });
});

describe("deriveMemoryDiff", () => {
  it("returns a stable empty diff when there is no candidate", () => {
    expect(deriveMemoryDiff(synthesis("Accepted", ["A"]), null)).toEqual({
      summaryChanged: false,
      added: [],
      removed: [],
    });
  });
});
