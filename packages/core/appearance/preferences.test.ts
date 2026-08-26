// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  APPEARANCE_IDS,
  DEFAULT_APPEARANCE,
  DEFAULT_SKIN,
  SKIN_IDS,
  changeAppearancePreferences,
  createAppearanceUndoReceipt,
  createDefaultAppearancePreferences,
  hasFutureAppearanceContractVersion,
  markAppearanceSyncFailed,
  markAppearanceSynced,
  nextAppearancePreferenceTimestamp,
  parseAppearancePreferences,
  reconcileAppearancePreferences,
  resetAppearancePreferences,
  resolveAppearance,
  undoAppearancePreferences,
  validateAppearancePreferences,
  withResolvedAppearance,
  type AppearancePreferences,
} from "./preferences";

const UPDATED_AT = "2026-08-23T10:00:00.000Z";

function preference(
  overrides: Partial<AppearancePreferences> = {},
): AppearancePreferences {
  return {
    version: APPEARANCE_PREFERENCES_VERSION,
    tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    skin: "tension",
    requestedAppearance: "system",
    resolvedAppearance: "dark",
    source: "local",
    updatedAt: UPDATED_AT,
    syncState: { status: "pending" },
    ...overrides,
  };
}

describe("appearance preference identifiers", () => {
  it("keeps the product contract deliberately bounded", () => {
    expect(SKIN_IDS).toEqual(["tension", "relay", "field"]);
    expect(APPEARANCE_IDS).toEqual(["system", "light", "dark"]);
    expect(DEFAULT_SKIN).toBe("tension");
    expect(DEFAULT_APPEARANCE).toBe("system");
    expect(APPEARANCE_PREFERENCES_VERSION).toBe(1);
    expect(APPEARANCE_TOKEN_CONTRACT_VERSION).toBe(1);
  });
});

describe("resolveAppearance", () => {
  it.each([
    ["light", "dark", "light"],
    ["dark", "light", "dark"],
    ["system", "light", "light"],
    ["system", "dark", "dark"],
  ] as const)("resolves %s against %s to %s", (requested, system, expected) => {
    expect(resolveAppearance(requested, system)).toBe(expected);
  });

  it("updates only the resolved value for an operating-system change", () => {
    const current = preference({
      requestedAppearance: "system",
      resolvedAppearance: "light",
      syncState: { status: "synced" },
    });

    expect(withResolvedAppearance(current, "dark")).toEqual({
      ...current,
      resolvedAppearance: "dark",
    });
  });
});

describe("appearance contract ordering", () => {
  it("recognizes only genuinely newer preference or token contracts", () => {
    expect(hasFutureAppearanceContractVersion({ version: 2 })).toBe(true);
    expect(
      hasFutureAppearanceContractVersion({ tokenContractVersion: 2 }),
    ).toBe(true);
    expect(
      hasFutureAppearanceContractVersion({
        version: APPEARANCE_PREFERENCES_VERSION,
        tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
      }),
    ).toBe(false);
    expect(hasFutureAppearanceContractVersion({ version: "2" })).toBe(false);
  });

  it("orders consecutive explicit choices inside the same millisecond", () => {
    const first = nextAppearancePreferenceTimestamp(
      "2026-08-23T10:00:00.000Z",
      Date.parse("2026-08-23T10:00:00.000Z"),
    );
    const second = nextAppearancePreferenceTimestamp(
      first,
      Date.parse("2026-08-23T10:00:00.000Z"),
    );

    expect(first).toBe("2026-08-23T10:00:00.001Z");
    expect(second).toBe("2026-08-23T10:00:00.002Z");
  });

  it("advances past a nanosecond timestamp without moving backwards", () => {
    expect(
      nextAppearancePreferenceTimestamp(
        "2026-08-23T10:00:00.123999999Z",
        Date.parse("2026-08-23T10:00:00.123Z"),
      ),
    ).toBe("2026-08-23T10:00:00.124Z");
  });
});

describe("parseAppearancePreferences", () => {
  it("validates a complete versioned value and derives resolved appearance", () => {
    const raw = preference({
      skin: "relay",
      requestedAppearance: "system",
      resolvedAppearance: "light",
      source: "server",
      syncState: { status: "synced" },
    });

    expect(parseAppearancePreferences(raw, { systemAppearance: "dark" })).toEqual({
      preferences: { ...raw, resolvedAppearance: "dark" },
      recovered: true,
      issues: [
        {
          field: "resolvedAppearance",
          code: "inconsistent_value",
        },
      ],
    });
  });

  it("recovers invalid fields without discarding valid independent choices", () => {
    const result = parseAppearancePreferences(
      {
        version: 1,
        tokenContractVersion: 99,
        skin: "unknown",
        requestedAppearance: "dark",
        resolvedAppearance: "chartreuse",
        source: "cookie",
        updatedAt: "yesterday",
        syncState: { status: "failed", errorClass: "the full server message" },
      },
      { systemAppearance: "light" },
    );

    expect(result.preferences).toEqual({
      version: 1,
      tokenContractVersion: 1,
      skin: "tension",
      requestedAppearance: "dark",
      resolvedAppearance: "dark",
      source: "default",
      updatedAt: "1970-01-01T00:00:00.000Z",
      syncState: { status: "local-only" },
    });
    expect(result.recovered).toBe(true);
    expect(result.issues.map((issue) => issue.field)).toEqual([
      "tokenContractVersion",
      "skin",
      "resolvedAppearance",
      "source",
      "updatedAt",
      "syncState",
    ]);
  });

  it("falls back atomically for an unsupported schema version", () => {
    const result = parseAppearancePreferences(
      { ...preference(), version: 2, skin: "field" },
      { systemAppearance: "light" },
    );

    expect(result.preferences).toEqual(
      createDefaultAppearancePreferences("light"),
    );
    expect(result.issues).toEqual([
      { field: "version", code: "unsupported_version" },
    ]);
  });

  it.each([
    ["2026-08-23T10:00:00.123456Z", "2026-08-23T10:00:00.123456Z"],
    ["2026-08-23T12:00:00+02:00", "2026-08-23T10:00:00.000Z"],
    ["2026-08-23T05:30:00.120000-04:30", "2026-08-23T10:00:00.120Z"],
  ])("accepts RFC3339 timestamps and normalizes %s", (updatedAt, expected) => {
    const result = parseAppearancePreferences(
      { ...preference(), updatedAt },
      { systemAppearance: "dark" },
    );

    expect(result.preferences.updatedAt).toBe(expected);
    expect(result.issues).toEqual([]);
  });

  it("treats an absent stored value as a clean default, not invalid recovery", () => {
    expect(parseAppearancePreferences(null, { systemAppearance: "dark" })).toEqual({
      preferences: createDefaultAppearancePreferences("dark"),
      recovered: false,
      issues: [],
    });
  });

  it("offers strict validation without silently replacing invalid input", () => {
    expect(validateAppearancePreferences(preference())).toEqual({
      valid: true,
      value: preference(),
    });
    expect(validateAppearancePreferences({ ...preference(), skin: "neon" })).toEqual({
      valid: false,
      issues: [{ field: "skin", code: "invalid_value" }],
    });
  });
});

describe("preference changes and sync state", () => {
  it("marks an explicit local change pending without treating resolution as a change", () => {
    const current = preference({ source: "server", syncState: { status: "synced" } });
    expect(
      changeAppearancePreferences(
        current,
        { skin: "field", requestedAppearance: "light" },
        { updatedAt: "2026-08-23T11:00:00.000Z", systemAppearance: "dark" },
      ),
    ).toEqual({
      ...current,
      skin: "field",
      requestedAppearance: "light",
      resolvedAppearance: "light",
      source: "local",
      updatedAt: "2026-08-23T11:00:00.000Z",
      syncState: { status: "pending" },
    });
  });

  it("normalizes an RFC3339 change timestamp before storing it", () => {
    const changed = changeAppearancePreferences(
      preference(),
      { skin: "field" },
      { updatedAt: "2026-08-23T13:00:00+02:00", systemAppearance: "dark" },
    );
    expect(changed.updatedAt).toBe("2026-08-23T11:00:00.000Z");
  });

  it("tracks only a bounded sync error class and can recover", () => {
    const failed = markAppearanceSyncFailed(preference(), "network");
    expect(failed.syncState).toEqual({ status: "failed", errorClass: "network" });
    expect(markAppearanceSynced(failed).syncState).toEqual({ status: "synced" });
  });

  it("resets both choices as one explicit local change", () => {
    expect(
      resetAppearancePreferences("2026-08-23T12:00:00.000Z", "dark"),
    ).toEqual({
      ...createDefaultAppearancePreferences("dark"),
      source: "local",
      updatedAt: "2026-08-23T12:00:00.000Z",
      syncState: { status: "pending" },
    });
  });

  it("captures the complete previous preference for a bounded Undo", () => {
    const previous = preference({
      skin: "relay",
      source: "server",
      syncState: { status: "synced" },
    });
    const applied = changeAppearancePreferences(
      previous,
      { skin: "field" },
      { updatedAt: "2026-08-23T11:00:00.000Z", systemAppearance: "dark" },
    );

    expect(createAppearanceUndoReceipt(previous, applied)).toEqual({
      previous,
      expectedUpdatedAt: "2026-08-23T11:00:00.000Z",
    });
  });

  it("restores both explicit choices as a new pending write", () => {
    const previous = preference({
      skin: "relay",
      requestedAppearance: "light",
      resolvedAppearance: "light",
      source: "server",
      syncState: { status: "synced" },
    });
    const applied = changeAppearancePreferences(
      previous,
      { skin: "field", requestedAppearance: "dark" },
      { updatedAt: "2026-08-23T11:00:00.000Z", systemAppearance: "dark" },
    );

    expect(
      undoAppearancePreferences(
        applied,
        createAppearanceUndoReceipt(previous, applied),
        { updatedAt: "2026-08-23T12:00:00.000Z", systemAppearance: "dark" },
      ),
    ).toEqual({
      status: "applied",
      expectedUpdatedAt: "2026-08-23T11:00:00.000Z",
      preferences: {
        ...applied,
        skin: "relay",
        requestedAppearance: "light",
        resolvedAppearance: "light",
        updatedAt: "2026-08-23T12:00:00.000Z",
      },
    });
  });

  it("expires without changing a newer local or external preference", () => {
    const previous = preference({ skin: "relay" });
    const applied = changeAppearancePreferences(
      previous,
      { skin: "field" },
      { updatedAt: "2026-08-23T11:00:00.000Z", systemAppearance: "dark" },
    );
    const newer = changeAppearancePreferences(
      applied,
      { skin: "tension" },
      { updatedAt: "2026-08-23T11:30:00.000Z", systemAppearance: "dark" },
    );

    expect(
      undoAppearancePreferences(
        newer,
        createAppearanceUndoReceipt(previous, applied),
        { updatedAt: "2026-08-23T12:00:00.000Z", systemAppearance: "dark" },
      ),
    ).toEqual({ status: "expired", preferences: newer });
  });
});

describe("reconcileAppearancePreferences", () => {
  const older = preference({ updatedAt: "2026-08-23T09:00:00.000Z" });
  const newer = preference({
    skin: "relay",
    source: "server",
    updatedAt: "2026-08-23T11:00:00.000Z",
    syncState: { status: "synced" },
  });

  it("uses the newer server choice and refreshes the local cache", () => {
    expect(reconcileAppearancePreferences(older, newer, "light")).toEqual({
      preferences: {
        ...newer,
        resolvedAppearance: "light",
        source: "server",
        syncState: { status: "synced" },
      },
      winner: "server",
      shouldPersistLocal: true,
      shouldSyncServer: false,
    });
  });

  it("keeps a newer local choice pending for server sync", () => {
    const result = reconcileAppearancePreferences(newer, older, "dark");
    expect(result.winner).toBe("local");
    expect(result.shouldSyncServer).toBe(true);
    expect(result.shouldPersistLocal).toBe(false);
    expect(result.preferences.syncState).toEqual({ status: "pending" });
  });

  it("uses local on an exact timestamp conflict so an explicit local edit is imported", () => {
    const server = preference({
      skin: "field",
      source: "server",
      syncState: { status: "synced" },
    });
    const result = reconcileAppearancePreferences(preference(), server, "dark");
    expect(result.winner).toBe("local");
    expect(result.shouldSyncServer).toBe(true);
  });

  it("preserves server microsecond ordering during reconciliation", () => {
    const local = preference({ updatedAt: "2026-08-23T10:00:00.123456Z" });
    const server = preference({
      skin: "field",
      source: "server",
      updatedAt: "2026-08-23T10:00:00.123789Z",
      syncState: { status: "synced" },
    });

    expect(reconcileAppearancePreferences(local, server, "dark").winner).toBe(
      "server",
    );
  });

  it("settles equal values as synced without an unnecessary write", () => {
    const server = preference({ source: "server", syncState: { status: "synced" } });
    const result = reconcileAppearancePreferences(preference(), server, "dark");
    expect(result.winner).toBe("server");
    expect(result.shouldSyncServer).toBe(false);
    expect(result.preferences.syncState).toEqual({ status: "synced" });
  });

  it("imports a lone explicit local preference but does not upload a clean default", () => {
    expect(
      reconcileAppearancePreferences(preference(), null, "dark").shouldSyncServer,
    ).toBe(true);
    expect(
      reconcileAppearancePreferences(
        createDefaultAppearancePreferences("dark"),
        null,
        "dark",
      ).shouldSyncServer,
    ).toBe(false);
  });
});
