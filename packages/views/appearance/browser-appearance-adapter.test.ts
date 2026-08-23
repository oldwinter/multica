import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  type AppearanceAdapterEvent,
  type AppearanceAdapterSource,
  type AppearancePreferenceAdapter,
  type AppearancePreferences,
} from "@multica/core/appearance";
import { defineAppearancePreferenceAdapterConformanceSuite } from "@multica/core/appearance/conformance";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyCachedBrowserAppearanceBeforePaint,
  BROWSER_APPEARANCE_OWNER_STORAGE_KEY,
  BROWSER_APPEARANCE_STORAGE_KEY,
  createBrowserAppearanceAdapter,
  LEGACY_APPEARANCE_STORAGE_KEY,
  LEGACY_SKIN_STORAGE_KEY,
} from "./browser-appearance-adapter";

type BrowserSource = Extract<
  AppearanceAdapterSource,
  "web" | "desktop" | "docs"
>;

function fixture(
  overrides: Partial<AppearancePreferences> = {},
): AppearancePreferences {
  return {
    version: APPEARANCE_PREFERENCES_VERSION,
    tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    skin: "tension",
    requestedAppearance: "system",
    resolvedAppearance: "dark",
    source: "local",
    updatedAt: "2026-08-23T10:00:00.000Z",
    syncState: { status: "pending" },
    ...overrides,
  };
}

function createBrowserHarness(source: BrowserSource) {
  localStorage.clear();
  let applied: AppearancePreferences | null = null;
  const listeners = new Set<(event: AppearanceAdapterEvent) => void>();
  const browserAdapter = createBrowserAppearanceAdapter(source);
  const adapter: AppearancePreferenceAdapter = {
    ...browserAdapter,
    apply(preferences) {
      applied = preferences;
      return browserAdapter.apply(preferences);
    },
    subscribe(listener) {
      listeners.add(listener);
      const unsubscribe = browserAdapter.subscribe(listener);
      return () => {
        listeners.delete(listener);
        unsubscribe();
      };
    },
  };

  return {
    create: () => adapter,
    seed: (value: unknown | null) => {
      if (value === null) {
        localStorage.removeItem(BROWSER_APPEARANCE_STORAGE_KEY);
      } else {
        localStorage.setItem(BROWSER_APPEARANCE_STORAGE_KEY, JSON.stringify(value));
      }
    },
    readPersisted: () => {
      const value = localStorage.getItem(BROWSER_APPEARANCE_STORAGE_KEY);
      return value === null ? null : (JSON.parse(value) as unknown);
    },
    readApplied: () => applied,
    emit: (event: AppearanceAdapterEvent) => {
      for (const listener of listeners) listener(event);
    },
  };
}

for (const source of ["web", "desktop", "docs"] as const) {
  defineAppearancePreferenceAdapterConformanceSuite(
    `${source} browser appearance adapter`,
    source,
    () => createBrowserHarness(source),
    { beforeEach, describe, expect, it },
  );
}

describe("browser appearance bootstrap", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    localStorage.clear();
    document.documentElement.className = "";
    document.documentElement.removeAttribute("data-skin");
    document.documentElement.style.colorScheme = "";
  });

  it("migrates legacy choices into the versioned contract", async () => {
    localStorage.setItem(LEGACY_SKIN_STORAGE_KEY, "field");
    localStorage.setItem(LEGACY_APPEARANCE_STORAGE_KEY, "dark");

    expect(await createBrowserAppearanceAdapter("web").load()).toMatchObject({
      version: APPEARANCE_PREFERENCES_VERSION,
      tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
      skin: "field",
      requestedAppearance: "dark",
      resolvedAppearance: "dark",
      source: "local",
      syncState: { status: "pending" },
    });
  });

  it("applies the validated cache before React creates its root", () => {
    localStorage.setItem(
      BROWSER_APPEARANCE_STORAGE_KEY,
      JSON.stringify({
        version: APPEARANCE_PREFERENCES_VERSION,
        tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
        skin: "relay",
        requestedAppearance: "dark",
        resolvedAppearance: "dark",
        source: "local",
        updatedAt: "2026-08-23T10:00:00.000Z",
        syncState: { status: "pending" },
      }),
    );

    applyCachedBrowserAppearanceBeforePaint();

    expect(document.documentElement.dataset.skin).toBe("relay");
    expect(document.documentElement).toHaveClass("dark");
    expect(document.documentElement.style.colorScheme).toBe("dark");
  });

  it("fails closed to product defaults when the cache is malformed", () => {
    localStorage.setItem(BROWSER_APPEARANCE_STORAGE_KEY, "not-json");

    applyCachedBrowserAppearanceBeforePaint();

    expect(document.documentElement.dataset.skin).toBe("tension");
    expect(document.documentElement).not.toHaveClass("dark");
  });

  it("keeps account caches isolated after the legacy cache is claimed", async () => {
    const adapter = createBrowserAppearanceAdapter("web");
    const accountA = {
      ...fixture(),
      skin: "relay" as const,
      updatedAt: "2026-08-23T10:00:00.000Z",
    };
    const accountB = {
      ...fixture(),
      skin: "field" as const,
      updatedAt: "2026-08-23T11:00:00.000Z",
    };

    await adapter.persistForAccount?.("account-a", accountA);
    expect(await adapter.loadForAccount?.("account-a")).toEqual(accountA);
    expect(await adapter.loadForAccount?.("account-b")).toBeNull();

    await adapter.persistForAccount?.("account-b", accountB);
    expect(await adapter.loadForAccount?.("account-a")).toEqual(accountA);
    expect(await adapter.loadForAccount?.("account-b")).toEqual(accountB);
    expect(
      localStorage.getItem(BROWSER_APPEARANCE_OWNER_STORAGE_KEY),
    ).toBe("account-b");
  });

  it("does not paint an appearance projection owned by another account", async () => {
    const adapter = createBrowserAppearanceAdapter("web");
    const accountA = fixture({ skin: "relay", resolvedAppearance: "dark" });
    const accountB = fixture({ skin: "field", resolvedAppearance: "light" });
    await adapter.persistForAccount?.("account-a", accountA);
    await adapter.persistForAccount?.("account-b", accountB);
    localStorage.setItem(BROWSER_APPEARANCE_STORAGE_KEY, JSON.stringify(accountA));
    localStorage.setItem(BROWSER_APPEARANCE_OWNER_STORAGE_KEY, "account-a");

    expect(await adapter.load()).toEqual(accountB);
    applyCachedBrowserAppearanceBeforePaint();

    expect(document.documentElement.dataset.skin).toBe("field");
    expect(document.documentElement).not.toHaveClass("dark");
  });

  it("imports an ownerless legacy choice for only the first account", async () => {
    localStorage.setItem(LEGACY_SKIN_STORAGE_KEY, "relay");
    localStorage.setItem(LEGACY_APPEARANCE_STORAGE_KEY, "dark");
    const adapter = createBrowserAppearanceAdapter("desktop");

    const imported = await adapter.loadForAccount?.("account-a");
    expect(imported).toMatchObject({
      skin: "relay",
      requestedAppearance: "dark",
      source: "local",
    });
    await adapter.persistForAccount?.(
      "account-a",
      imported as AppearancePreferences,
    );

    expect(await adapter.loadForAccount?.("account-b")).toBeNull();
  });

  it("labels account-scoped storage events for cross-tab isolation", () => {
    const adapter = createBrowserAppearanceAdapter("web");
    const listener = vi.fn();
    const unsubscribe = adapter.subscribe(listener);
    const value = fixture({ skin: "field" });

    window.dispatchEvent(
      new StorageEvent("storage", {
        key: `${BROWSER_APPEARANCE_STORAGE_KEY}:account:account-b`,
        newValue: JSON.stringify(value),
      }),
    );

    expect(listener).toHaveBeenCalledWith({
      type: "external-preferences-changed",
      value,
      accountId: "account-b",
    });
    unsubscribe();
  });

  it("requests account reconciliation when a browser surface becomes active", () => {
    const adapter = createBrowserAppearanceAdapter("web");
    const listener = vi.fn();
    const unsubscribe = adapter.subscribe(listener);

    window.dispatchEvent(new FocusEvent("focus"));

    expect(listener).toHaveBeenCalledWith({
      type: "connectivity-changed",
      online: navigator.onLine,
    });
    unsubscribe();
  });

  it("does not overwrite a future cache during pre-paint fallback", () => {
    const raw = JSON.stringify({
      ...fixture(),
      tokenContractVersion: 2,
      futureSentinel: "keep-me",
    });
    localStorage.setItem(BROWSER_APPEARANCE_STORAGE_KEY, raw);

    applyCachedBrowserAppearanceBeforePaint();

    expect(localStorage.getItem(BROWSER_APPEARANCE_STORAGE_KEY)).toBe(raw);
    expect(localStorage.getItem(LEGACY_SKIN_STORAGE_KEY)).toBeNull();
    expect(document.documentElement.dataset.skin).toBe("tension");
  });

  it("surfaces storage access failures while pre-paint remains usable", () => {
    const getItem = vi
      .spyOn(Storage.prototype, "getItem")
      .mockImplementation(() => {
        throw new DOMException("blocked", "SecurityError");
      });
    const adapter = createBrowserAppearanceAdapter("web");

    expect(() => adapter.load()).toThrow("blocked");
    expect(() => applyCachedBrowserAppearanceBeforePaint()).not.toThrow();
    expect(document.documentElement.dataset.skin).toBe("tension");
    getItem.mockRestore();

    const setItem = vi
      .spyOn(Storage.prototype, "setItem")
      .mockImplementation(() => {
        throw new DOMException("full", "QuotaExceededError");
      });
    expect(() => adapter.persist(fixture())).toThrow("full");
    setItem.mockRestore();
  });
});
