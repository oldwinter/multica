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
    await client.createRoom({ title: "Research room", facilitator_agent_id: "agent-1" });
    await client.postRoomMessage("room-1", {
      body: "Investigate this.",
      idempotency_key: "message-key-1",
    });
    await client.wakeRoom("room-1", { idempotency_key: "wake-key-1" });
    await client.setRoomStatus("room-1", { status: "paused" });
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

  it("rejects a malformed create response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ room: "wrong" })));

    await expect(
      new ApiClient("https://api.example.test").createRoom({
        title: "Research room",
        facilitator_agent_id: "agent-1",
      }),
    ).rejects.toThrow();
  });

  it("rejects a malformed message response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ cycle: "wrong" })));

    await expect(
      new ApiClient("https://api.example.test").postRoomMessage("room-1", {
        body: "Investigate this.",
        idempotency_key: "message-key-1",
      }),
    ).rejects.toThrow();
  });

  it("rejects a malformed wake response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ cycle: "wrong" })));

    await expect(
      new ApiClient("https://api.example.test").wakeRoom("room-1", {
        idempotency_key: "wake-key-1",
      }),
    ).rejects.toThrow();
  });

  it("rejects a malformed status response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ status: "paused" })));

    await expect(
      new ApiClient("https://api.example.test").setRoomStatus("room-1", { status: "paused" }),
    ).rejects.toThrow();
  });

  it("rejects a malformed promotion response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ kind: "decision" })));

    await expect(
      new ApiClient("https://api.example.test").promoteRoomArtifact("room-1", {
        entry_id: "entry-1",
        kind: "decision",
        idempotency_key: "promotion-key-1",
        title: "Choose the primary source",
      }),
    ).rejects.toThrow();
  });
});

function jsonResponse(value: unknown): Response {
  return new Response(JSON.stringify(value), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
