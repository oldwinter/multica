import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { EMPTY_ROOM_DETAIL } from "./schemas";

const room = {
  id: "room-1",
  workspace_id: "workspace-1",
  title: "Research room",
  instructions: "Discuss the evidence.",
  created_by_user_id: "user-1",
  facilitator_agent_id: "agent-1",
  facilitator_squad_id: null,
  status: "active",
  daily_turn_limit: null,
  schedule_interval_minutes: null,
  next_wake_at: null,
  active_cycle_id: null,
  memory: {},
  memory_version: 0,
  created_at: "2026-08-13T00:00:00Z",
  updated_at: "2026-08-13T00:00:00Z",
};

const cycle = {
  id: "cycle-1",
  sequence: 1,
  source: "manual",
  wake_key: "manual:key-1",
  triggering_entry_id: null,
  status: "queued",
  refusal_reason: null,
  planned_at: null,
  created_at: "2026-08-13T00:00:00Z",
  started_at: null,
  completed_at: null,
};

const turn = {
  id: "turn-1",
  cycle_id: "cycle-1",
  agent_id: "agent-1",
  squad_id: null,
  status: "queued",
  refusal_reason: null,
  created_at: "2026-08-13T00:00:00Z",
  started_at: null,
  completed_at: null,
};

const detail = {
  room,
  participants: [],
  entries: [],
  cycles: [cycle],
  turns: [turn],
  artifacts: [],
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("ApiClient rooms", () => {
  it("uses the Room endpoints with the caller's idempotency keys", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse([room]))
      .mockResolvedValueOnce(jsonResponse(detail))
      .mockResolvedValueOnce(jsonResponse(detail))
      .mockResolvedValueOnce(jsonResponse({ cycle, turns: [turn], tasks: ["task-1"], entry: {
        id: "entry-1",
        cycle_id: "cycle-1",
        turn_id: "turn-1",
        ordinal: 1,
        type: "message",
        author_type: "member",
        author_id: "user-1",
        body: "Investigate this.",
        mentions: [],
        created_at: "2026-08-13T00:00:00Z",
      } }))
      .mockResolvedValueOnce(jsonResponse({ cycle, turns: [turn], tasks: ["task-1"] }))
      .mockResolvedValueOnce(jsonResponse({ ...room, status: "paused" }))
			.mockResolvedValueOnce(jsonResponse({ ...room, daily_turn_limit: 20, max_cost_ticks: 200 }))
      .mockResolvedValueOnce(jsonResponse({
        id: "artifact-1",
        cycle_id: null,
        turn_id: "turn-1",
        entry_id: "entry-1",
        kind: "decision",
        target_id: "artifact-1",
        title: "Choose the primary source",
        body: "Evidence favours A.",
        rationale: null,
        created_by_user_id: "user-1",
        created_at: "2026-08-13T00:00:00Z",
      }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await client.listRooms();
    await client.getRoom("room-1");
    await client.createRoom({
      title: "Research room",
      objective: "Compare the evidence",
      facilitator_agent_id: "agent-1",
    });
    await client.postRoomMessage("room-1", {
      body: "Investigate this.",
      idempotency_key: "message-key-1",
    });
    await client.wakeRoom("room-1", { idempotency_key: "wake-key-1" });
    await client.setRoomStatus("room-1", { status: "paused" });
		await client.updateRoomBudget("room-1", {
			daily_turn_limit: 20,
			max_cost_ticks: 200,
		});
    await client.promoteRoomArtifact("room-1", {
      entry_id: "entry-1",
      kind: "decision",
      idempotency_key: "promotion-key-1",
      title: "Choose the primary source",
    });

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      "https://api.example.test/api/rooms",
      "https://api.example.test/api/rooms/room-1",
      "https://api.example.test/api/rooms",
      "https://api.example.test/api/rooms/room-1/messages",
      "https://api.example.test/api/rooms/room-1/wake",
      "https://api.example.test/api/rooms/room-1/status",
			"https://api.example.test/api/rooms/room-1/budget",
      "https://api.example.test/api/rooms/room-1/promotions",
    ]);
    expect(fetchMock.mock.calls[3]?.[1]).toMatchObject({
      method: "POST",
      body: expect.stringContaining('"idempotency_key":"message-key-1"'),
    });
    expect(fetchMock.mock.calls[4]?.[1]).toMatchObject({
      method: "POST",
      body: expect.stringContaining('"idempotency_key":"wake-key-1"'),
    });
		expect(fetchMock.mock.calls[6]?.[1]).toMatchObject({
			method: "PUT",
			body: JSON.stringify({ daily_turn_limit: 20, max_cost_ticks: 200 }),
		});
		expect(fetchMock.mock.calls[7]?.[1]).toMatchObject({
      method: "POST",
      body: expect.stringContaining('"idempotency_key":"promotion-key-1"'),
    });
  });

  it("degrades malformed Room detail responses to an empty detail", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ room: "wrong" })));

    await expect(new ApiClient("https://api.example.test").getRoom("room-1")).resolves.toEqual(
      EMPTY_ROOM_DETAIL,
    );
  });

  it("degrades malformed Room list responses to an empty list", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ rooms: [] })));

    await expect(new ApiClient("https://api.example.test").listRooms()).resolves.toEqual([]);
  });

	it("degrades a malformed budget update response", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ id: 42 })));

		await expect(
			new ApiClient("https://api.example.test").updateRoomBudget("room-1", {
				daily_turn_limit: null,
				max_cost_ticks: null,
			}),
		).resolves.toMatchObject({ id: "", daily_turn_limit: null, max_cost_ticks: null });
	});

  it("parses authoritative preflight and usage responses", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({
        allowed: true,
        refusal_reason: null,
        capability_version: 2,
        capability_ready: true,
        required_daemon_capability: "rooms_outcome_v2",
				spend_limit_supported: true,
				required_cost_capability: "room-cost-limits-v1",
        target_agents: [{
          agent_id: "agent-1",
          ready: true,
          invocation_allowed: true,
          reason: null,
        }],
        expected_max_turns: 2,
        synthesis_required: true,
        budget: {
          daily_turn_limit: 10,
          used_turns: 2,
          max_cost_ticks: 100,
          used_cost_ticks: 20,
          remaining_cost_ticks: 80,
					reserved_cost_ticks: 10,
          uncosted_turns: 0,
        },
      }))
      .mockResolvedValueOnce(jsonResponse({
        turns_total: 8,
        cost_ticks: 64,
        uncosted_turns: 1,
        failures: 1,
        accepted_syntheses: 2,
        promoted_artifacts: 1,
      }));
    vi.stubGlobal("fetch", fetchMock);
    const client = new ApiClient("https://api.example.test");

    await expect(client.getRoomPreflight("room-1", "agent-1", "schedule")).resolves.toMatchObject({
      source: "unknown",
      allowed: true,
      capability_version: 2,
      capability_ready: true,
      required_daemon_capability: "rooms_outcome_v2",
			spend_limit_supported: true,
			required_cost_capability: "room-cost-limits-v1",
      expected_max_turns: 2,
    });
    await expect(client.getRoomUsage("room-1")).resolves.toMatchObject({
      turns_total: 8,
      accepted_syntheses: 2,
    });
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      "https://api.example.test/api/rooms/room-1/preflight?source=schedule&target_agent_id=agent-1",
      "https://api.example.test/api/rooms/room-1/usage",
    ]);
  });

  it("degrades malformed preflight and usage responses", async () => {
    vi.stubGlobal("fetch", vi.fn()
      .mockResolvedValueOnce(jsonResponse({ allowed: "yes" }))
      .mockResolvedValueOnce(jsonResponse({ turns_total: "many" })));
    const client = new ApiClient("https://api.example.test");

    await expect(client.getRoomPreflight("room-1")).resolves.toMatchObject({
      allowed: false,
      capability_version: 0,
      capability_ready: true,
      target_agents: [],
    });
    await expect(client.getRoomUsage("room-1")).resolves.toMatchObject({
      turns_total: 0,
      promoted_artifacts: 0,
    });
  });

  it("falls back for a malformed create response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ room: "wrong" })));

    await expect(
      new ApiClient("https://api.example.test").createRoom({
        title: "Research room",
        objective: "Compare the evidence",
        facilitator_agent_id: "agent-1",
      }),
    ).resolves.toMatchObject({ room: { id: "" }, entries: [] });
  });

  it("falls back for a malformed message response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ cycle: "wrong" })));

    await expect(
      new ApiClient("https://api.example.test").postRoomMessage("room-1", {
        body: "Investigate this.",
        idempotency_key: "message-key-1",
      }),
    ).resolves.toMatchObject({ entry: { id: "" }, cycle: { id: "" } });
  });

  it("falls back for a malformed wake response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ cycle: "wrong" })));

    await expect(
      new ApiClient("https://api.example.test").wakeRoom("room-1", {
        idempotency_key: "wake-key-1",
      }),
    ).resolves.toMatchObject({ cycle: { id: "" }, turns: [], tasks: [] });
  });

  it("falls back for a malformed status response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ status: "paused" })));

    await expect(
      new ApiClient("https://api.example.test").setRoomStatus("room-1", { status: "paused" }),
    ).resolves.toMatchObject({ id: "", status: "unknown" });
  });

  it("falls back for a malformed promotion response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ kind: "decision" })));

    await expect(
      new ApiClient("https://api.example.test").promoteRoomArtifact("room-1", {
        entry_id: "entry-1",
        kind: "decision",
        idempotency_key: "promotion-key-1",
        title: "Choose the primary source",
      }),
    ).resolves.toMatchObject({ id: "", kind: "decision" });
  });
});

function jsonResponse(value: unknown): Response {
  return new Response(JSON.stringify(value), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
