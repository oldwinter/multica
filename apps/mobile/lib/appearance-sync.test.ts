// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  type AppearancePreferences,
} from "@multica/core/appearance";
import {
  classifyAppearanceSyncError,
  readServerAppearance,
  toAppearanceUpdateRequest,
} from "./appearance-sync";

describe("mobile server appearance boundary", () => {
  it("treats an older server without the additive tuple as no preference", () => {
    expect(readServerAppearance({ id: "user-1" }, "dark")).toEqual({
      preferences: null,
      writable: true,
      recoveredFields: [],
    });
  });

  it("parses a microsecond server tuple and resolves system locally", () => {
    const result = readServerAppearance(
      {
        skin: "relay",
        appearance: "system",
        appearanceUpdatedAt: "2026-08-23T10:00:00.123456Z",
        appearanceTokenVersion: 1,
      },
      "light",
    );

    expect(result.writable).toBe(true);
    expect(result.preferences).toMatchObject({
      skin: "relay",
      requestedAppearance: "system",
      resolvedAppearance: "light",
      updatedAt: "2026-08-23T10:00:00.123456Z",
      source: "server",
      syncState: { status: "synced" },
    });
  });

  it("fails closed instead of overwriting a future token contract", () => {
    expect(
      readServerAppearance(
        {
          skin: "field",
          appearance: "dark",
          appearanceUpdatedAt: "2026-08-23T10:00:00.000Z",
          appearanceTokenVersion: 2,
        },
        "light",
      ),
    ).toEqual({
      preferences: null,
      writable: false,
      recoveredFields: ["tokenContractVersion"],
    });
  });

  it("recovers malformed current-version fields independently", () => {
    const result = readServerAppearance(
      {
        skin: "neon",
        appearance: "dark",
        appearanceUpdatedAt: "yesterday",
        appearanceTokenVersion: 1,
      },
      "light",
    );

    expect(result.writable).toBe(true);
    expect(result.preferences).toMatchObject({
      skin: "tension",
      requestedAppearance: "dark",
      resolvedAppearance: "dark",
      updatedAt: "1970-01-01T00:00:00.000Z",
    });
    expect(result.recoveredFields).toEqual(["skin", "updatedAt"]);
  });

  it("writes all account-sync fields as one tuple", () => {
    const preferences: AppearancePreferences = {
      version: APPEARANCE_PREFERENCES_VERSION,
      tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
      skin: "field",
      requestedAppearance: "dark",
      resolvedAppearance: "dark",
      source: "local",
      updatedAt: "2026-08-23T10:00:00.000Z",
      syncState: { status: "pending" },
    };
    expect(toAppearanceUpdateRequest(preferences)).toEqual({
      skin: "field",
      appearance: "dark",
      appearanceUpdatedAt: "2026-08-23T10:00:00.000Z",
      appearanceTokenVersion: 1,
    });
  });
});

describe("mobile appearance sync errors", () => {
  it.each([
    [{ status: 401 }, "unauthorized"],
    [{ status: 409 }, "conflict"],
    [{ status: 503 }, "server"],
    [new TypeError("Network request failed"), "network"],
    [new Error("unexpected"), "unknown"],
  ] as const)("bounds %o as %s", (error, expected) => {
    expect(classifyAppearanceSyncError(error)).toBe(expected);
  });
});
