// @vitest-environment node

import { ApiError } from "@multica/core/api";
import { describe, expect, it } from "vitest";
import { roomMessageWasPersisted } from "./rooms-page";

describe("roomMessageWasPersisted", () => {
  it("treats an unsupported spend limit as a persisted message", () => {
    const error = new ApiError("Spend limit is unsupported", 409, "Conflict", {
      code: "spend_limit_unsupported",
    });

    expect(roomMessageWasPersisted(error)).toBe(true);
  });
});
