/** @vitest-environment jsdom */
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import { issueKeys } from "../issues/queries";
import { wikiKeys } from "../wiki/queries";
import {
  useCreateRoom,
  usePostRoomMessage,
  usePromoteRoomArtifact,
  useSetRoomStatus,
  useWakeRoom,
} from "./mutations";
import { roomKeys } from "./queries";
import type { Room, RoomArtifact, RoomDetail, RoomMessageResult, RoomWakeResult } from "./types";

vi.mock("../hooks", () => ({ useWorkspaceId: () => "ws-1" }));

const room: Room = {
  id: "room-1",
  workspace_id: "ws-1",
  title: "Research room",
  instructions: "",
  created_by_user_id: "user-1",
  facilitator_agent_id: "agent-1",
  facilitator_squad_id: null,
  status: "active",
  daily_turn_limit: null,
  schedule_interval_minutes: null,
  next_wake_at: null,
  active_cycle_id: null,
  memory: {
    summary: "",
    facts: [],
    decisions: [],
    open_questions: [],
    recent_contributions: [],
  },
  memory_version: 0,
  created_at: "2026-08-13T00:00:00Z",
  updated_at: "2026-08-13T00:00:00Z",
};

const detail: RoomDetail = {
  room,
  participants: [],
  entries: [],
  cycles: [],
  turns: [],
  artifacts: [],
};

const artifact: RoomArtifact = {
  id: "artifact-1",
  cycle_id: null,
  turn_id: "turn-1",
  entry_id: "entry-1",
  kind: "issue",
  target_id: "issue-1",
  title: "Promoted issue",
  body: "Body",
  rationale: null,
  created_by_user_id: "user-1",
  created_at: "2026-08-13T00:00:00Z",
};

function wrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function createQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function roomMutationResponses() {
  return {
    createRoom: vi.fn<() => Promise<RoomDetail>>().mockResolvedValue(detail),
    postRoomMessage: vi.fn<() => Promise<RoomMessageResult>>(),
    wakeRoom: vi.fn<() => Promise<RoomWakeResult>>(),
    setRoomStatus: vi.fn<() => Promise<Room>>().mockResolvedValue(room),
    promoteRoomArtifact: vi.fn<() => Promise<RoomArtifact>>().mockResolvedValue(artifact),
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("Room mutations", () => {
  it("invalidates the full Room cache tree after create, post, wake, and status changes", async () => {
    const queryClient = createQueryClient();
    const api = roomMutationResponses();
    api.postRoomMessage.mockResolvedValue({
      cycle: {
        id: "cycle-1",
        sequence: 1,
        source: "message",
        wake_key: "message:key-1",
        triggering_entry_id: "entry-1",
        status: "queued",
        refusal_reason: null,
        planned_at: null,
        created_at: "2026-08-13T00:00:00Z",
        started_at: null,
        completed_at: null,
      },
      turns: [],
      tasks: [],
      entry: {
        id: "entry-1",
        cycle_id: "cycle-1",
        turn_id: null,
        ordinal: 1,
        type: "message",
        author_type: "member",
        author_id: "user-1",
        body: "Discuss this.",
        mentions: [],
        created_at: "2026-08-13T00:00:00Z",
      },
    });
    api.wakeRoom.mockResolvedValue({
      cycle: {
        id: "cycle-2",
        sequence: 2,
        source: "manual",
        wake_key: "manual:key-2",
        triggering_entry_id: null,
        status: "queued",
        refusal_reason: null,
        planned_at: null,
        created_at: "2026-08-13T00:00:00Z",
        started_at: null,
        completed_at: null,
      },
      turns: [],
      tasks: [],
    });
    setApiInstance(api as unknown as ApiClient);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const options = { wrapper: wrapper(queryClient) };
    const create = renderHook(() => useCreateRoom(), options);
    const post = renderHook(() => usePostRoomMessage("room-1"), options);
    const wake = renderHook(() => useWakeRoom("room-1"), options);
    const status = renderHook(() => useSetRoomStatus("room-1"), options);

    await act(async () => {
      await create.result.current.mutateAsync({ title: "Research room", facilitator_agent_id: "agent-1" });
      await post.result.current.mutateAsync({ body: "Discuss this.", idempotency_key: "message-key-1" });
      await wake.result.current.mutateAsync({ idempotency_key: "wake-key-1" });
      await status.result.current.mutateAsync({ status: "paused" });
    });

    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: roomKeys.all("ws-1") }),
    );
    expect(invalidate).toHaveBeenCalledTimes(4);
    queryClient.clear();
  });

  it("invalidates the promoted Issue or Wiki cache plus the Room cache", async () => {
    const queryClient = createQueryClient();
    const api = roomMutationResponses();
    setApiInstance(api as unknown as ApiClient);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => usePromoteRoomArtifact("room-1"), {
      wrapper: wrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({
        entry_id: "entry-1",
        kind: "issue",
        idempotency_key: "issue-key-1",
        title: "Promoted issue",
      });
    });
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: issueKeys.all("ws-1") }),
    );
    expect(invalidate).toHaveBeenCalledWith({ queryKey: roomKeys.all("ws-1") });

    api.promoteRoomArtifact.mockResolvedValue({ ...artifact, kind: "wiki", target_id: "wiki-1" });
    await act(async () => {
      await result.current.mutateAsync({
        entry_id: "entry-1",
        kind: "wiki",
        idempotency_key: "wiki-key-1",
        title: "Promoted wiki",
      });
    });
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: wikiKeys.all("ws-1") }),
    );
    queryClient.clear();
  });
});
