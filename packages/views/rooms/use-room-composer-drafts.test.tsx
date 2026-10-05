// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { setCurrentWorkspace } from "@multica/core/platform";
import {
  ROOM_COMPOSER_DRAFT_STORAGE_KEY,
  useRoomComposerDraftStore,
} from "@multica/core/rooms";

const authRef = vi.hoisted(() => ({ userId: "user-a" as string | null }));
const workspaceRef = vi.hoisted(() => ({ id: "workspace-shared" }));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (
    selector: (state: { user: { id: string } | null }) => unknown,
  ) => selector({ user: authRef.userId ? { id: authRef.userId } : null }),
}));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => workspaceRef,
}));

import { useRoomComposerDrafts } from "./use-room-composer-drafts";

const flush = () => new Promise((resolve) => queueMicrotask(() => resolve(null)));

describe("useRoomComposerDrafts session isolation", () => {
  beforeEach(async () => {
    authRef.userId = "user-a";
    workspaceRef.id = "workspace-shared";
    await act(async () => {
      setCurrentWorkspace("shared", workspaceRef.id);
      await flush();
      localStorage.clear();
      useRoomComposerDraftStore.getState().clearDraft();
      useRoomComposerDraftStore.getState().setDraft({
        ownerUserId: "user-a",
        ownerWorkspaceId: workspaceRef.id,
        rooms: {
          "room-private": {
            body: "Alice's unsent message",
            mentionAgentIds: ["agent-private"],
            idempotencyKey: "alice-key",
            status: "idle",
          },
        },
      });
    });
  });

  afterEach(async () => {
    await act(async () => {
      setCurrentWorkspace(null, null);
      await flush();
    });
  });

  it("hides the previous user's draft across auth loss and a new login", async () => {
    const { result, rerender } = renderHook(() =>
      useRoomComposerDrafts("room-private"),
    );
    expect(result.current.draft?.body).toBe("Alice's unsent message");
    const completeForUserA = result.current.complete;
    const markFailedForUserA = result.current.markFailed;
    act(() => {
      result.current.markPending("room-private", "alice-key");
    });

    act(() => {
      authRef.userId = null;
      rerender();
    });
    expect(result.current.draft).toBeUndefined();

    act(() => {
      authRef.userId = "user-b";
      rerender();
    });
    expect(result.current.draft?.body ?? "").not.toBe("Alice's unsent message");

    await waitFor(() => {
      expect(useRoomComposerDraftStore.getState().draft).toMatchObject({
        ownerUserId: "user-b",
        ownerWorkspaceId: "workspace-shared",
      });
    });
    expect(
      useRoomComposerDraftStore.getState().draft.rooms["room-private"]?.body,
    ).toBe("");

    act(() => {
      result.current.updateBody("room-private", "Bob's unsent message");
    });
    expect(result.current.draft?.body).toBe("Bob's unsent message");

    act(() => {
      completeForUserA("room-private", "alice-key");
      markFailedForUserA("room-private", "alice-key");
    });
    expect(useRoomComposerDraftStore.getState().draft).toMatchObject({
      ownerUserId: "user-b",
      ownerWorkspaceId: "workspace-shared",
      rooms: {
        "room-private": {
          body: "Bob's unsent message",
          status: "idle",
        },
      },
    });
  });

  it("does not let an old workspace hydration listener overwrite the new workspace", async () => {
    const { result, rerender } = renderHook(() =>
      useRoomComposerDrafts("room-private"),
    );
    expect(result.current.draft?.body).toBe("Alice's unsent message");

    localStorage.setItem(
      `${ROOM_COMPOSER_DRAFT_STORAGE_KEY}:beta`,
      JSON.stringify({
        state: {
          draft: {
            ownerUserId: "user-a",
            ownerWorkspaceId: "workspace-beta",
            rooms: {
              "room-private": {
                body: "Beta workspace draft",
                mentionAgentIds: [],
                idempotencyKey: "beta-key",
                status: "idle",
              },
            },
          },
        },
        version: 0,
      }),
    );

    await act(async () => {
      // Keep the old hook rendered while rehydration completes to reproduce
      // the render/microtask/passive-cleanup ordering used by app layouts.
      setCurrentWorkspace("beta", "workspace-beta");
      await flush();
    });

    expect(useRoomComposerDraftStore.getState().draft).toMatchObject({
      ownerUserId: "user-a",
      ownerWorkspaceId: "workspace-beta",
      rooms: {
        "room-private": { body: "Beta workspace draft" },
      },
    });

    workspaceRef.id = "workspace-beta";
    rerender();
    expect(result.current.draft?.body).toBe("Beta workspace draft");
  });
});
