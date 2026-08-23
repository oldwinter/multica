// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineAppearancePreferenceAdapterConformanceSuite } from "@multica/core/appearance/conformance";
import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  type AppearanceAdapterEvent,
  type AppearancePreferences,
  type ResolvedAppearance,
} from "@multica/core/appearance";
import {
  APPEARANCE_STORAGE_KEY,
  appearanceAccountStorageKey,
  hasFutureAppearanceTokenVersion,
  persistMobileAppearanceCache,
  SKIN_STORAGE_KEY,
  selectAccountAppearanceValue,
  THEME_STORAGE_KEY,
  createMobileAppearanceAdapter,
} from "./appearance-preferences";

function createHarness() {
  const storage = new Map<string, string>();
  let applied: AppearancePreferences | null = null;
  let systemAppearance: ResolvedAppearance = "dark";
  const systemListeners = new Set<(value: ResolvedAppearance) => void>();
  const connectivityListeners = new Set<(value: boolean) => void>();

  const adapter = createMobileAppearanceAdapter({
    storage: {
      readItem: (key) => storage.get(key) ?? null,
      writeItem: (key, value) => {
        storage.set(key, value);
      },
    },
    platform: {
      apply: (preferences) => {
        applied = preferences;
      },
      getSystemAppearance: () => systemAppearance,
      getReducedMotion: () => false,
      getForcedColors: () => false,
      getOnline: () => true,
      subscribeSystemAppearance: (listener) => {
        systemListeners.add(listener);
        return () => systemListeners.delete(listener);
      },
      subscribeConnectivity: (listener) => {
        connectivityListeners.add(listener);
        return () => connectivityListeners.delete(listener);
      },
    },
  });

  return {
    create: () => adapter,
    seed: (value: unknown | null) => {
      if (value === null) storage.delete(APPEARANCE_STORAGE_KEY);
      else storage.set(APPEARANCE_STORAGE_KEY, JSON.stringify(value));
    },
    readPersisted: () => {
      const serialized = storage.get(APPEARANCE_STORAGE_KEY);
      return serialized ? (JSON.parse(serialized) as unknown) : null;
    },
    readApplied: () => applied,
    emit: (event: AppearanceAdapterEvent) => {
      if (event.type === "connectivity-changed") {
        for (const listener of connectivityListeners) listener(event.online);
      }
      if (event.type === "system-appearance-changed") {
        systemAppearance = event.systemAppearance;
        for (const listener of systemListeners) listener(event.systemAppearance);
      }
    },
    storage,
  };
}

defineAppearancePreferenceAdapterConformanceSuite(
  "mobile SecureStore appearance adapter",
  "mobile",
  createHarness,
  { beforeEach, describe, expect, it },
);

describe("mobile appearance storage migration", () => {
  it("imports legacy skin and appearance as one explicit pending tuple", async () => {
    const harness = createHarness();
    harness.storage.set(SKIN_STORAGE_KEY, "relay");
    harness.storage.set(THEME_STORAGE_KEY, "light");

    await expect(
      Promise.resolve((await harness.create()).load()),
    ).resolves.toEqual({
      version: APPEARANCE_PREFERENCES_VERSION,
      tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
      skin: "relay",
      requestedAppearance: "light",
      resolvedAppearance: "light",
      source: "local",
      updatedAt: "1970-01-01T00:00:00.000Z",
      syncState: { status: "pending" },
    });
  });

  it("prefers the full versioned value over stale legacy keys", async () => {
    const harness = createHarness();
    const current: AppearancePreferences = {
      version: APPEARANCE_PREFERENCES_VERSION,
      tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
      skin: "field",
      requestedAppearance: "dark",
      resolvedAppearance: "dark",
      source: "server",
      updatedAt: "2026-08-23T12:00:00.000Z",
      syncState: { status: "synced" },
    };
    harness.storage.set(APPEARANCE_STORAGE_KEY, JSON.stringify(current));
    harness.storage.set(SKIN_STORAGE_KEY, "relay");
    harness.storage.set(THEME_STORAGE_KEY, "light");

    await expect(
      Promise.resolve((await harness.create()).load()),
    ).resolves.toEqual(current);
  });
});

describe("mobile account appearance cache", () => {
  it("uses the signed-in account cache instead of another account's bootstrap", () => {
    const bootstrap = { skin: "relay" };
    const account = { skin: "field" };

    expect(selectAccountAppearanceValue(account, bootstrap, "user-a")).toBe(
      account,
    );
    expect(selectAccountAppearanceValue(null, bootstrap, "user-a")).toBeNull();
  });

  it("imports an ownerless legacy bootstrap once", () => {
    const bootstrap = { skin: "relay" };
    expect(selectAccountAppearanceValue(null, bootstrap, null)).toBe(bootstrap);
  });

  it("builds collision-free account keys accepted by SecureStore", () => {
    const first = appearanceAccountStorageKey("user:a/é");
    const second = appearanceAccountStorageKey("user_a/é");

    expect(first).toMatch(/^[A-Za-z0-9._-]+$/);
    expect(first).not.toBe(second);
  });

  it("detects only newer local token contracts as future values", () => {
    expect(hasFutureAppearanceTokenVersion({ tokenContractVersion: 2 })).toBe(
      true,
    );
    expect(hasFutureAppearanceTokenVersion({ tokenContractVersion: 1 })).toBe(
      false,
    );
    expect(hasFutureAppearanceTokenVersion({ tokenContractVersion: "2" })).toBe(
      false,
    );
  });

  it("never overwrites a future local token value", async () => {
    const storage = new Map<string, string>([
      [APPEARANCE_STORAGE_KEY, JSON.stringify({ tokenContractVersion: 2 })],
    ]);
    const persistBootstrap = vi.fn();

    await expect(
      persistMobileAppearanceCache({
        storage: {
          readItem: (key) => storage.get(key) ?? null,
          writeItem: (key, value) => {
            storage.set(key, value);
          },
        },
        preferences: {
          version: APPEARANCE_PREFERENCES_VERSION,
          tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
          skin: "tension",
          requestedAppearance: "system",
          resolvedAppearance: "dark",
          source: "default",
          updatedAt: "1970-01-01T00:00:00.000Z",
          syncState: { status: "failed", errorClass: "conflict" },
        },
        accountId: "user-a",
        writable: false,
        persistBootstrap,
      }),
    ).resolves.toBe(false);

    expect(persistBootstrap).not.toHaveBeenCalled();
    expect(storage).toEqual(
      new Map([
        [APPEARANCE_STORAGE_KEY, JSON.stringify({ tokenContractVersion: 2 })],
      ]),
    );
  });
});
