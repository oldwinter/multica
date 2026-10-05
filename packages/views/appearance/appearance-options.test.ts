// @vitest-environment node

import {
  APPEARANCE_IDS,
  SKIN_IDS,
  type AppearancePreferences,
} from "@multica/core/appearance";
import { describe, expect, it } from "vitest";
import {
  APPEARANCE_OPTIONS,
  getAppearanceSyncMessage,
  isRequestedAppearance,
  isSkinId,
  SKIN_OPTIONS,
} from "./appearance-options";

const preference = (
  overrides: Partial<AppearancePreferences> = {},
): AppearancePreferences => ({
  version: 1,
  tokenContractVersion: 1,
  skin: "tension",
  requestedAppearance: "system",
  resolvedAppearance: "light",
  source: "local",
  updatedAt: "2026-01-01T00:00:00.000Z",
  syncState: { status: "synced" },
  ...overrides,
});

describe("appearance option registry", () => {
  it("keeps the UI order aligned with the shared domain values", () => {
    expect(SKIN_OPTIONS.map((option) => option.value)).toEqual([...SKIN_IDS]);
    expect(APPEARANCE_OPTIONS.map((option) => option.value)).toEqual([
      ...APPEARANCE_IDS,
    ]);
  });

  it("guards values at the UI boundary", () => {
    expect(isSkinId("relay")).toBe(true);
    expect(isSkinId("unknown")).toBe(false);
    expect(isRequestedAppearance("dark")).toBe(true);
    expect(isRequestedAppearance(null)).toBe(false);
  });
});

describe("appearance sync message", () => {
  it("distinguishes default preferences from saved local preferences", () => {
    expect(getAppearanceSyncMessage(preference({
      source: "default", syncState: { status: "local-only" },
    }))).toBe("default");
    expect(getAppearanceSyncMessage(preference({
      syncState: { status: "local-only" },
    }))).toBe("local_only");
  });

  it("keeps pending and failed synchronization visible for default preferences", () => {
    expect(getAppearanceSyncMessage(preference({
      source: "default", syncState: { status: "pending" },
    }))).toBe("pending");
    expect(getAppearanceSyncMessage(preference({
      source: "default", syncState: { status: "failed", errorClass: "storage" },
    }))).toBe("failed");
  });
});
