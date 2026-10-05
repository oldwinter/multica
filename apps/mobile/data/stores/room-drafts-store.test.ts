// @vitest-environment node

import { beforeEach, describe, expect, it } from "vitest";
import { roomDraftKey, useRoomDraftsStore } from "./room-drafts-store";

describe("Room draft idempotency", () => {
  beforeEach(() => {
    useRoomDraftsStore.setState({ drafts: {} });
  });

  it("keeps one idempotency key while the draft changes", () => {
    const key = roomDraftKey("ws-1", "room-1");
    useRoomDraftsStore.getState().setBody(key, "First");
    const firstKey = useRoomDraftsStore.getState().drafts[key]?.idempotencyKey;

    useRoomDraftsStore.getState().setBody(key, "First, revised");

    expect(useRoomDraftsStore.getState().drafts[key]?.idempotencyKey).toBe(
      firstKey,
    );
  });

  it("rotates the key when an ambiguous submission is edited", () => {
    const key = roomDraftKey("ws-1", "room-1");
    const store = useRoomDraftsStore.getState();
    store.setBody(key, "Submitted body");
    const firstKey = useRoomDraftsStore.getState().drafts[key]?.idempotencyKey;
    useRoomDraftsStore.getState().markSubmitted(key, "Submitted body");

    useRoomDraftsStore.getState().setBody(key, "Changed after timeout");

    const changed = useRoomDraftsStore.getState().drafts[key];
    expect(changed?.idempotencyKey).not.toBe(firstKey);
    expect(changed?.submittedBody).toBeNull();
  });

  it("keeps the key while retrying the identical submitted payload", () => {
    const key = roomDraftKey("ws-1", "room-1");
    useRoomDraftsStore.getState().setBody(key, "Retry me");
    const firstKey = useRoomDraftsStore.getState().drafts[key]?.idempotencyKey;
    useRoomDraftsStore.getState().markSubmitted(key, "Retry me");

    useRoomDraftsStore.getState().setBody(key, "Retry me");

    expect(useRoomDraftsStore.getState().drafts[key]?.idempotencyKey).toBe(firstKey);
  });

  it("starts a new logical message after a successful clear", () => {
    const key = roomDraftKey("ws-1", "room-1");
    useRoomDraftsStore.getState().setBody(key, "First");
    const firstKey = useRoomDraftsStore.getState().drafts[key]?.idempotencyKey;
    useRoomDraftsStore.getState().clear(key);
    useRoomDraftsStore.getState().setBody(key, "Second");

    expect(useRoomDraftsStore.getState().drafts[key]?.idempotencyKey).not.toBe(
      firstKey,
    );
  });
});
