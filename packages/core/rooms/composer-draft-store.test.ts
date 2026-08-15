// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { resetAllRegisteredDrafts } from "../drafts/cleanup-registry";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import {
  ROOM_COMPOSER_DRAFT_STORAGE_KEY,
  useRoomComposerDraftStore,
} from "./composer-draft-store";

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
      "room-a": {
        body: "recover me",
        mentionAgentIds: ["agent-a"],
        idempotencyKey: "key-a",
        status: "idle",
      },
    });

    const stored = localStorage.getItem(`${ROOM_COMPOSER_DRAFT_STORAGE_KEY}:acme`);
    expect(stored).not.toBeNull();
    expect(JSON.parse(stored ?? "{}").state.draft["room-a"]).toMatchObject({
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
            "room-b": {
              body: "retry after reload",
              mentionAgentIds: ["agent-b"],
              idempotencyKey: "stable-key",
              status: "pending",
            },
          },
        },
        version: 0,
      }),
    );
    setCurrentWorkspace("beta", "workspace-b");
    await flush();

    expect(useRoomComposerDraftStore.getState().draft["room-b"]).toEqual({
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
      "room-c": {
        body: "private note",
        mentionAgentIds: [],
        idempotencyKey: "key-c",
        status: "idle",
      },
    });

    resetAllRegisteredDrafts();

    expect(useRoomComposerDraftStore.getState().draft).toEqual({});
    expect(useRoomComposerDraftStore.getState().hasDraft()).toBe(false);
  });
});
