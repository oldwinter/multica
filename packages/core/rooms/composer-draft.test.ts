import { describe, expect, it } from "vitest";
import {
  EMPTY_ROOM_COMPOSER_DRAFTS,
  completeRoomComposerDraft,
  ensureRoomComposerDraft,
  markRoomComposerFailed,
  markRoomComposerPending,
  updateRoomComposerBody,
  updateRoomComposerMention,
} from "./composer-draft";

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
    expect(roomB["room-b"]?.idempotencyKey).toBe("key-b");
  });

  it("reuses the same idempotency key when a failed draft is retried", () => {
    const initial = ensureRoomComposerDraft(
      EMPTY_ROOM_COMPOSER_DRAFTS,
      "room-a",
      "stable-key",
    );
    const withBody = updateRoomComposerBody(initial, "room-a", "retry me", "unused");
    const failed = markRoomComposerFailed(
      markRoomComposerPending(withBody, "room-a"),
      "room-a",
    );
    const retried = markRoomComposerPending(failed, "room-a");

    expect(retried["room-a"]).toMatchObject({
      body: "retry me",
      idempotencyKey: "stable-key",
      status: "pending",
    });
  });

  it("rotates the idempotency key when failed content changes", () => {
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
    const changedBody = updateRoomComposerBody(
      failed,
      "room-a",
      "new body",
      "body-key",
    );
    const failedAgain = markRoomComposerFailed(
      markRoomComposerPending(changedBody, "room-a"),
      "room-a",
    );
    const changedMention = updateRoomComposerMention(
      failedAgain,
      "room-a",
      "agent-a",
      true,
      "mention-key",
    );

    expect(changedBody["room-a"]).toMatchObject({
      idempotencyKey: "body-key",
      status: "idle",
    });
    expect(changedMention["room-a"]).toMatchObject({
      mentionAgentIds: ["agent-a"],
      idempotencyKey: "mention-key",
      status: "idle",
    });
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
