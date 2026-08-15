// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { resetAllRegisteredDrafts } from "../drafts/cleanup-registry";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import {
  ROOM_COMPOSER_DRAFT_STORAGE_KEY,
  useRoomComposerDraftStore,
} from "./composer-draft-store";
import { roomComposerDraftsForScope } from "./composer-draft";

const flush = () => new Promise((resolve) => queueMicrotask(() => resolve(null)));

beforeAll(() => {
  if (typeof globalThis.localStorage?.clear !== "function") {
    const values = new Map<string, string>();
    const storage: Storage = {
      get length() {
        return values.size;
      },
      clear: () => values.clear(),
      getItem: (key) => values.get(key) ?? null,
      key: (index) => Array.from(values.keys())[index] ?? null,
      removeItem: (key) => {
        values.delete(key);
      },
      setItem: (key, value) => {
        values.set(key, value);
      },
    };
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      value: storage,
    });
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: storage,
    });
  }
});

describe("room composer draft store", () => {
  beforeEach(async () => {
    setCurrentWorkspace(null, null);
    await flush();
    localStorage.clear();
    useRoomComposerDraftStore.getState().clearDraft();
  });

  afterEach(async () => {
    setCurrentWorkspace(null, null);
    await flush();
  });

  it("persists room-keyed drafts inside the active workspace namespace", async () => {
    setCurrentWorkspace("acme", "workspace-a");
    await flush();
    useRoomComposerDraftStore.getState().setDraft({
      ownerUserId: "user-a",
      ownerWorkspaceId: "workspace-a",
      rooms: {
        "room-a": {
          body: "recover me",
          mentionAgentIds: ["agent-a"],
          idempotencyKey: "key-a",
          status: "idle",
        },
      },
    });

    const stored = localStorage.getItem(`${ROOM_COMPOSER_DRAFT_STORAGE_KEY}:acme`);
    expect(stored).not.toBeNull();
    expect(JSON.parse(stored ?? "{}").state.draft.rooms["room-a"]).toMatchObject({
      body: "recover me",
      mentionAgentIds: ["agent-a"],
      idempotencyKey: "key-a",
    });
    expect(useRoomComposerDraftStore.getState().hasDraft()).toBe(true);
  });

  it("restores an interrupted pending request as a retryable failed draft", async () => {
    localStorage.setItem(
      `${ROOM_COMPOSER_DRAFT_STORAGE_KEY}:beta`,
      JSON.stringify({
        state: {
          draft: {
            ownerUserId: "user-b",
            ownerWorkspaceId: "workspace-b",
            rooms: {
              "room-b": {
                body: "retry after reload",
                mentionAgentIds: ["agent-b"],
                idempotencyKey: "stable-key",
                status: "pending",
              },
            },
          },
        },
        version: 0,
      }),
    );
    setCurrentWorkspace("beta", "workspace-b");
    await flush();

    expect(useRoomComposerDraftStore.getState().draft.rooms["room-b"]).toEqual({
      body: "retry after reload",
      mentionAgentIds: ["agent-b"],
      idempotencyKey: "stable-key",
      status: "failed",
    });
  });

  it("clears in-memory intent on logout cleanup", async () => {
    setCurrentWorkspace("gamma", "workspace-c");
    await flush();
    useRoomComposerDraftStore.getState().setDraft({
      ownerUserId: "user-c",
      ownerWorkspaceId: "workspace-c",
      rooms: {
        "room-c": {
          body: "private note",
          mentionAgentIds: [],
          idempotencyKey: "key-c",
          status: "idle",
        },
      },
    });

    resetAllRegisteredDrafts();

    expect(useRoomComposerDraftStore.getState().draft).toEqual({
      ownerUserId: null,
      ownerWorkspaceId: null,
      rooms: {},
    });
    expect(useRoomComposerDraftStore.getState().hasDraft()).toBe(false);
  });

  it("does not expose one user's draft to another user in the same workspace", async () => {
    setCurrentWorkspace("shared", "workspace-shared");
    await flush();
    useRoomComposerDraftStore.getState().setDraft({
      ownerUserId: "user-a",
      ownerWorkspaceId: "workspace-shared",
      rooms: {
        "room-private": {
          body: "Alice only",
          mentionAgentIds: ["agent-private"],
          idempotencyKey: "alice-key",
          status: "idle",
        },
      },
    });

    const stored = useRoomComposerDraftStore.getState().draft;
    expect(
      roomComposerDraftsForScope(stored, {
        userId: "user-b",
        workspaceId: "workspace-shared",
      }),
    ).toEqual({});
    expect(
      roomComposerDraftsForScope(stored, {
        userId: "user-a",
        workspaceId: "workspace-shared",
      })["room-private"]?.body,
    ).toBe("Alice only");
  });

  it("does not expose drafts when a workspace slug is reused", async () => {
    setCurrentWorkspace("reused", "workspace-old");
    await flush();
    useRoomComposerDraftStore.getState().setDraft({
      ownerUserId: "user-a",
      ownerWorkspaceId: "workspace-old",
      rooms: {
        "room-old": {
          body: "Old workspace",
          mentionAgentIds: [],
          idempotencyKey: "old-key",
          status: "idle",
        },
      },
    });

    expect(
      roomComposerDraftsForScope(useRoomComposerDraftStore.getState().draft, {
        userId: "user-a",
        workspaceId: "workspace-new",
      }),
    ).toEqual({});
  });
});
