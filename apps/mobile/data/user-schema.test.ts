// @vitest-environment node

import { describe, expect, it } from "vitest";
import { UserSchema } from "./schemas";

describe("mobile UserSchema appearance compatibility", () => {
  it("defaults the additive tuple for an older server", () => {
    expect(UserSchema.parse({ id: "user-1" })).toMatchObject({
      id: "user-1",
      skin: null,
      appearance: null,
      appearanceUpdatedAt: null,
      appearanceTokenVersion: null,
    });
  });

  it("keeps a valid versioned tuple", () => {
    expect(
      UserSchema.parse({
        id: "user-1",
        skin: "relay",
        appearance: "system",
        appearance_updated_at: "2026-08-23T10:00:00.123456Z",
        appearance_token_version: 1,
      }),
    ).toMatchObject({
      skin: "relay",
      appearance: "system",
      appearanceUpdatedAt: "2026-08-23T10:00:00.123456Z",
      appearanceTokenVersion: 1,
    });
  });

  it("isolates malformed additive fields from required identity", () => {
    expect(
      UserSchema.parse({
        id: "user-1",
        name: "Ada",
        skin: "neon",
        appearance: "sepia",
        appearance_updated_at: 42,
        appearance_token_version: "one",
      }),
    ).toMatchObject({
      id: "user-1",
      name: "Ada",
      skin: null,
      appearance: null,
      appearanceUpdatedAt: null,
      appearanceTokenVersion: null,
    });
  });
});
