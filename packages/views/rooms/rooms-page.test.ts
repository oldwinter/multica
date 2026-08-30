// @vitest-environment node

import { ApiError } from "@multica/core/api";
import { describe, expect, it } from "vitest";
import { isLinkedRoomMissing, roomMessageWasPersisted } from "./rooms-page";

describe("roomMessageWasPersisted", () => {
  it("treats an unsupported spend limit as a persisted message", () => {
    const error = new ApiError("Spend limit is unsupported", 409, "Conflict", {
      code: "spend_limit_unsupported",
    });

    expect(roomMessageWasPersisted(error)).toBe(true);
  });
});

describe("isLinkedRoomMissing", () => {
  it("does not replace an unresolved deep link with another Room", () => {
    expect(isLinkedRoomMissing("room-missing", [{ id: "room-a" }], true)).toBe(true);
    expect(isLinkedRoomMissing("room-a", [{ id: "room-a" }], true)).toBe(false);
    expect(isLinkedRoomMissing("room-missing", [], false)).toBe(false);
  });
});
