import { describe, expect, it } from "vitest";
import {
  EMPTY_ROOM_COMPOSER_DRAFTS,
  completeRoomComposerDraft,
  ensureRoomComposerDraft,
  markRoomComposerFailed,
  markRoomComposerPending,
  updateRoomComposerBody,
  updateRoomComposerMention,
} from "./room-composer-draft";

describe("room composer drafts", () => {
  it("keeps message content and mentions isolated by room", () => {
    const roomA = ensureRoomComposerDraft(
      EMPTY_ROOM_COMPOSER_DRAFTS,
      "room-a",
      "key-a",
    );
    const withBody = updateRoomComposerBody(roomA, "room-a", "alpha", "unused");
    const withMention = updateRoomComposerMention(
      withBody,
      "room-a",
      "agent-a",
      true,
      "unused",
    );

    const roomB = ensureRoomComposerDraft(withMention, "room-b", "key-b");

    expect(roomB["room-a"]).toEqual({
      body: "alpha",
      mentionAgentIds: ["agent-a"],
      idempotencyKey: "key-a",
      status: "idle",
    });
    expect(roomB["room-b"]).toEqual({
      body: "",
      mentionAgentIds: [],
      idempotencyKey: "key-b",
      status: "idle",
    });
  });

  it("reuses the same idempotency key when a failed draft is retried", () => {
    const initial = ensureRoomComposerDraft(
      EMPTY_ROOM_COMPOSER_DRAFTS,
      "room-a",
      "stable-key",
    );
    const withBody = updateRoomComposerBody(initial, "room-a", "retry me", "unused");
    const pending = markRoomComposerPending(withBody, "room-a");
    const failed = markRoomComposerFailed(pending, "room-a");

    const retried = markRoomComposerPending(failed, "room-a");

    expect(retried["room-a"]?.idempotencyKey).toBe("stable-key");
    expect(retried["room-a"]?.body).toBe("retry me");
    expect(retried["room-a"]?.status).toBe("pending");
  });

  it("rotates the idempotency key when a failed request becomes a new draft", () => {
    const initial = ensureRoomComposerDraft(
      EMPTY_ROOM_COMPOSER_DRAFTS,
      "room-a",
      "old-key",
    );
    const withBody = updateRoomComposerBody(initial, "room-a", "old body", "unused");
    const failed = markRoomComposerFailed(
      markRoomComposerPending(withBody, "room-a"),
      "room-a",
    );

    const changed = updateRoomComposerBody(failed, "room-a", "new body", "new-key");

    expect(changed["room-a"]?.idempotencyKey).toBe("new-key");
    expect(changed["room-a"]?.status).toBe("idle");
  });

  it("rotates the idempotency key when mentions change after failure", () => {
    const initial = ensureRoomComposerDraft(
      EMPTY_ROOM_COMPOSER_DRAFTS,
      "room-a",
      "old-key",
    );
    const failed = markRoomComposerFailed(
      markRoomComposerPending(initial, "room-a"),
      "room-a",
    );

    const changed = updateRoomComposerMention(
      failed,
      "room-a",
      "agent-a",
      true,
      "new-key",
    );

    expect(changed["room-a"]?.mentionAgentIds).toEqual(["agent-a"]);
    expect(changed["room-a"]?.idempotencyKey).toBe("new-key");
    expect(changed["room-a"]?.status).toBe("idle");
  });

  it("clears a successful draft and rotates its idempotency key", () => {
    const initial = ensureRoomComposerDraft(
      EMPTY_ROOM_COMPOSER_DRAFTS,
      "room-a",
      "old-key",
    );
    const withBody = updateRoomComposerBody(initial, "room-a", "done", "unused");

    const completed = completeRoomComposerDraft(withBody, "room-a", "next-key");

    expect(completed["room-a"]).toEqual({
      body: "",
      mentionAgentIds: [],
      idempotencyKey: "next-key",
      status: "idle",
    });
  });
});
