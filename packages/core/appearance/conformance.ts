import type {
  AppearanceAdapterEvent,
  AppearanceAdapterSource,
  AppearancePreferenceAdapter,
  Awaitable,
} from "./adapter";
import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  changeAppearancePreferences,
  parseAppearancePreferences,
  reconcileAppearancePreferences,
  resetAppearancePreferences,
  type AppearancePreferences,
} from "./preferences";
import { createAppearanceDiagnostics } from "./diagnostics";

export interface AppearancePreferenceAdapterConformanceHarness {
  create(): Awaitable<AppearancePreferenceAdapter>;
  seed(value: unknown | null): Awaitable<void>;
  readPersisted(): Awaitable<unknown | null>;
  readApplied(): Awaitable<AppearancePreferences | null>;
  emit(event: AppearanceAdapterEvent): Awaitable<void>;
}

export type AppearancePreferenceAdapterConformanceHarnessFactory =
  () => AppearancePreferenceAdapterConformanceHarness;

export interface AppearanceConformanceTestApi {
  describe(name: string, suite: () => void): unknown;
  beforeEach(setup: () => void | Promise<void>): unknown;
  it(name: string, test: () => void | Promise<void>): unknown;
  expect(actual: unknown): {
    toBe(expected: unknown): unknown;
    toEqual(expected: unknown): unknown;
    toContainEqual(expected: unknown): unknown;
  };
}

function fixture(
  overrides: Partial<AppearancePreferences> = {},
): AppearancePreferences {
  return {
    version: APPEARANCE_PREFERENCES_VERSION,
    tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    skin: "relay",
    requestedAppearance: "system",
    resolvedAppearance: "dark",
    source: "local",
    updatedAt: "2026-08-23T10:00:00.000Z",
    syncState: { status: "pending" },
    ...overrides,
  };
}

/**
 * Shared behavioral suite for every platform adapter. The harness owns only
 * platform mechanics; validation, conflict handling, reset, and diagnostics
 * are exercised through the same core functions real callers use.
 */
export function defineAppearancePreferenceAdapterConformanceSuite(
  name: string,
  expectedSource: AppearanceAdapterSource,
  createHarness: AppearancePreferenceAdapterConformanceHarnessFactory,
  testApi: AppearanceConformanceTestApi,
): void {
  const { beforeEach, describe, expect, it } = testApi;

  describe(name, () => {
    let harness: AppearancePreferenceAdapterConformanceHarness;
    let adapter: AppearancePreferenceAdapter;

    beforeEach(async () => {
      harness = createHarness();
      adapter = await harness.create();
    });

    it("declares its stable platform source and remote-sync capability", () => {
      expect(adapter.source).toBe(expectedSource);
      expect(typeof adapter.supportsRemoteSync).toBe("boolean");
    });

    it("loads untrusted storage through validation and invalid-value recovery", async () => {
      await harness.seed({ ...fixture(), skin: "not-a-skin" });
      const parsed = parseAppearancePreferences(await adapter.load(), {
        systemAppearance: "dark",
      });
      expect(parsed.recovered).toBe(true);
      expect(parsed.preferences.skin).toBe("tension");
      expect(parsed.issues).toContainEqual({
        field: "skin",
        code: "invalid_value",
      });
    });

    it("persists and applies a valid preference without changing its tuple", async () => {
      const preferences = fixture();
      await adapter.persist(preferences);
      await adapter.apply(preferences);
      expect(await harness.readPersisted()).toEqual(preferences);
      expect(await harness.readApplied()).toEqual(preferences);
    });

    it("supports an offline local change and reconnect notification", async () => {
      const seen: AppearanceAdapterEvent[] = [];
      const unsubscribe = adapter.subscribe((event) => seen.push(event));
      await harness.emit({ type: "connectivity-changed", online: false });

      const changed = changeAppearancePreferences(
        fixture({ syncState: { status: "synced" } }),
        { skin: "field" },
        {
          updatedAt: "2026-08-23T11:00:00.000Z",
          systemAppearance: "dark",
        },
      );
      await adapter.persist(changed);
      expect(changed.syncState).toEqual({ status: "pending" });

      await harness.emit({ type: "connectivity-changed", online: true });
      unsubscribe();
      await harness.emit({ type: "connectivity-changed", online: false });
      expect(seen).toEqual([
        { type: "connectivity-changed", online: false },
        { type: "connectivity-changed", online: true },
      ]);
    });

    it("resolves system changes without manufacturing an explicit preference write", async () => {
      const environment = await adapter.getEnvironment();
      const parsed = parseAppearancePreferences(fixture(), {
        systemAppearance: environment.systemAppearance,
      });
      const diagnostics = createAppearanceDiagnostics(parsed.preferences, {
        adapterSource: adapter.source,
        reducedMotion: environment.reducedMotion,
        forcedColors: environment.forcedColors,
      });
      expect(diagnostics.resolvedAppearance).toBe(environment.systemAppearance);
      expect(diagnostics.adapterSource).toBe(expectedSource);
    });

    it("uploads one equal-timestamp conflict and settles the acknowledged tuple", () => {
      const local = fixture({ skin: "relay" });
      const conflictingServer = fixture({
        skin: "field",
        source: "server",
        syncState: { status: "synced" },
      });
      const conflict = reconcileAppearancePreferences(
        local,
        conflictingServer,
        "dark",
      );
      expect(conflict.winner).toBe("local");
      expect(conflict.shouldSyncServer).toBe(true);

      const acknowledgedServer = {
        ...conflict.preferences,
        source: "server" as const,
        syncState: { status: "synced" as const },
      };
      const settled = reconcileAppearancePreferences(
        conflict.preferences,
        acknowledgedServer,
        "dark",
      );
      expect(settled.winner).toBe("server");
      expect(settled.shouldSyncServer).toBe(false);
    });

    it("can reset, persist, and apply the product default as one operation", async () => {
      const reset = resetAppearancePreferences(
        "2026-08-23T12:00:00.000Z",
        "dark",
      );
      await adapter.persist(reset);
      await adapter.apply(reset);
      expect(await harness.readPersisted()).toEqual(reset);
      expect(await harness.readApplied()).toEqual(reset);
    });
  });
}
