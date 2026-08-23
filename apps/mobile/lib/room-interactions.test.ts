// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  canPromoteRoomRevision,
  RoomIdempotencyKeys,
  roomConflictCode,
  roomMessageWasSaved,
  roomErrorMessage,
  roomReviewCorrection,
} from "./room-interactions";

function apiError(status: number, code: string) {
  return Object.assign(new Error("server message"), {
    status,
    body: { code },
  });
}

describe("Room mutation interaction outcomes", () => {
  it.each([
    "room_paused",
    "room_archived",
    "budget_exhausted",
    "active_cycle",
    "agent_unavailable",
  ])(
    "clears the draft when 409 %s confirms the message was saved",
    (code) => {
      const error = apiError(409, code);
      expect(roomConflictCode(error)).toBe(code);
      expect(roomMessageWasSaved(error)).toBe(true);
      expect(roomErrorMessage(error)).toContain("Message saved");
    },
  );

  it("retains the draft for a permission refusal", () => {
    const error = apiError(409, "invocation_not_allowed");
    expect(roomMessageWasSaved(error)).toBe(false);
    expect(roomErrorMessage(error)).toContain("permission");
  });
});

describe("Room operation idempotency", () => {
  it("reuses a key for the same operation fingerprint", () => {
    let sequence = 0;
    const keys = new RoomIdempotencyKeys((action) => `${action}:${++sequence}`);

    expect(keys.keyFor("review", "cycle-1:accept")).toBe("review:1");
    expect(keys.keyFor("review", "cycle-1:accept")).toBe("review:1");
    expect(keys.keyFor("review", "cycle-1:corrected")).toBe("review:2");
  });

  it("rotates a key only after confirmed success", () => {
    let sequence = 0;
    const keys = new RoomIdempotencyKeys((action) => `${action}:${++sequence}`);
    const first = keys.keyFor("promotion", "revision-1:key-1:payload");

    keys.complete("promotion", "revision-1:key-1:payload");

    expect(keys.keyFor("promotion", "revision-1:key-1:payload")).not.toBe(first);
  });
});

describe("Room review gates", () => {
  const synthesis = {
    schema_version: 1,
    summary: "Original",
    facts: [{ text: "Fact", citation_entry_ids: ["entry-1"], confidence: 0.9 }],
    decisions: [],
    open_questions: [],
    disagreements: [],
    action_items: [],
    recommendations: [],
    confidence: 0.8,
  } as const;

  it("sends a complete corrected synthesis while preserving cited structure", () => {
    expect(roomReviewCorrection("correct", synthesis, " Corrected ")).toEqual({
      ...synthesis,
      summary: "Corrected",
    });
  });

  it("does not send correction text for a synthesis rejection", () => {
    expect(roomReviewCorrection("reject", synthesis, "discarded text")).toBeUndefined();
  });

  it.each(["pending", "corrected", "rejected", "unknown"] as const)(
    "blocks promotion from a %s revision",
    (status) => expect(canPromoteRoomRevision(status)).toBe(false),
  );

  it("allows promotion only from an accepted revision", () => {
    expect(canPromoteRoomRevision("accepted")).toBe(true);
  });
});
