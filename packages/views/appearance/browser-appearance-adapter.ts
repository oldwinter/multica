import {
  APPEARANCE_EPOCH,
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  DEFAULT_APPEARANCE,
  DEFAULT_SKIN,
  createDefaultAppearancePreferences,
  hasFutureAppearanceContractVersion,
  parseAppearancePreferences,
  type AppearanceAdapterListener,
  type AppearanceAdapterSource,
  type AppearancePreferenceAdapter,
  type AppearancePreferences,
  type ResolvedAppearance,
} from "@multica/core/appearance";
import { AUTHENTICATED_ACCOUNT_STORAGE_KEY } from "@multica/core/auth";

export const BROWSER_APPEARANCE_STORAGE_KEY =
  "multica-appearance-preferences";
export const BROWSER_APPEARANCE_OWNER_STORAGE_KEY =
  "multica-appearance-preferences-owner";
export const LEGACY_SKIN_STORAGE_KEY = "multica-skin";
export const LEGACY_APPEARANCE_STORAGE_KEY = "theme";

type BrowserAdapterSource = Extract<
  AppearanceAdapterSource,
  "web" | "desktop" | "docs"
>;

function currentWindow(): Window | null {
  return typeof window === "undefined" ? null : window;
}

function currentDocument(): Document | null {
  return typeof document === "undefined" ? null : document;
}

function readSystemAppearance(win: Window | null): ResolvedAppearance {
  return win?.matchMedia?.("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

function legacyPreferences(win: Window): unknown {
  const skinValue = win.localStorage.getItem(LEGACY_SKIN_STORAGE_KEY);
  const appearanceValue = win.localStorage.getItem(
    LEGACY_APPEARANCE_STORAGE_KEY,
  );
  const hasExplicitChoice = skinValue !== null || appearanceValue !== null;
  const requestedAppearance = appearanceValue ?? DEFAULT_APPEARANCE;
  const systemAppearance = readSystemAppearance(win);
  return {
    version: APPEARANCE_PREFERENCES_VERSION,
    tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    // Preserve invalid legacy values until the shared parser can report and
    // recover them through the bounded diagnostics/analytics contract.
    skin: skinValue ?? DEFAULT_SKIN,
    requestedAppearance,
    resolvedAppearance:
      requestedAppearance === "light" || requestedAppearance === "dark"
        ? requestedAppearance
        : systemAppearance,
    source: hasExplicitChoice ? "local" : "default",
    updatedAt: APPEARANCE_EPOCH,
    syncState: hasExplicitChoice
      ? { status: "pending" }
      : { status: "local-only" },
  };
}

function parseStoredValue(value: string): unknown {
  try {
    return JSON.parse(value) as unknown;
  } catch {
    return "";
  }
}

function readStoredValue(win: Window): unknown | null {
  const value = win.localStorage.getItem(BROWSER_APPEARANCE_STORAGE_KEY);
  return value === null ? legacyPreferences(win) : parseStoredValue(value);
}

function accountStorageKey(accountId: string): string {
  return `${BROWSER_APPEARANCE_STORAGE_KEY}:account:${encodeURIComponent(accountId)}`;
}

function accountIdFromStorageKey(key: string): string | null {
  const prefix = `${BROWSER_APPEARANCE_STORAGE_KEY}:account:`;
  if (!key.startsWith(prefix)) return null;
  try {
    return decodeURIComponent(key.slice(prefix.length));
  } catch {
    return null;
  }
}

function readStoredAccountValue(
  win: Window,
  accountId: string,
): unknown | null {
  const accountValue = win.localStorage.getItem(accountStorageKey(accountId));
  if (accountValue !== null) return parseStoredValue(accountValue);

  const owner = win.localStorage.getItem(
    BROWSER_APPEARANCE_OWNER_STORAGE_KEY,
  );
  // An ownerless cache is legacy data and may be imported by the first
  // authenticated account only. Once claimed, another account starts clean.
  if (owner === null || owner === accountId) return readStoredValue(win);
  return null;
}

function safeRead(win: Window): unknown | null {
  return readStoredValue(win);
}

function readBootstrapValue(win: Window): unknown | null {
  const owner = win.localStorage.getItem(
    BROWSER_APPEARANCE_OWNER_STORAGE_KEY,
  );
  const activeAccount = win.localStorage.getItem(
    AUTHENTICATED_ACCOUNT_STORAGE_KEY,
  );
  // Bootstrap can run before React has restored the authenticated user. Use
  // the durable account identity so hydration reads the same tuple as the
  // synchronous pre-paint script.
  return activeAccount
    ? readStoredAccountValue(win, activeAccount)
    : owner === null
      ? safeRead(win)
      : null;
}

function safePersist(win: Window, preferences: AppearancePreferences): void {
  win.localStorage.setItem(
    BROWSER_APPEARANCE_STORAGE_KEY,
    JSON.stringify(preferences),
  );
  // Keep installed clients that predate the versioned cache flicker-free.
  win.localStorage.setItem(LEGACY_SKIN_STORAGE_KEY, preferences.skin);
  win.localStorage.setItem(
    LEGACY_APPEARANCE_STORAGE_KEY,
    preferences.requestedAppearance,
  );
}

function safePersistForAccount(
  win: Window,
  accountId: string,
  preferences: AppearancePreferences,
): void {
  const serialized = JSON.stringify(preferences);
  // Claim the scoped cache before updating the bootstrap projection. A quota
  // failure therefore cannot make another account inherit this tuple.
  win.localStorage.setItem(accountStorageKey(accountId), serialized);
  win.localStorage.setItem(AUTHENTICATED_ACCOUNT_STORAGE_KEY, accountId);
  win.localStorage.setItem(BROWSER_APPEARANCE_OWNER_STORAGE_KEY, accountId);
  safePersist(win, preferences);
}

function applyToDocument(
  doc: Document,
  preferences: AppearancePreferences,
): void {
  const root = doc.documentElement;
  root.dataset.skin = preferences.skin;
  root.classList.toggle("dark", preferences.resolvedAppearance === "dark");
  root.style.colorScheme = preferences.resolvedAppearance;
}

/**
 * DOM/localStorage adapter shared mechanically by browser-backed platforms.
 * The explicit source keeps diagnostics and analytics platform-specific.
 */
export function createBrowserAppearanceAdapter(
  source: BrowserAdapterSource,
): AppearancePreferenceAdapter {
  return {
    source,
    supportsRemoteSync: source !== "docs",
    load() {
      const win = currentWindow();
      return win ? readBootstrapValue(win) : null;
    },
    loadForAccount(accountId) {
      const win = currentWindow();
      return win ? readStoredAccountValue(win, accountId) : null;
    },
    persist(preferences) {
      const win = currentWindow();
      if (win) safePersist(win, preferences);
    },
    persistForAccount(accountId, preferences) {
      const win = currentWindow();
      if (win) safePersistForAccount(win, accountId, preferences);
    },
    apply(preferences) {
      const doc = currentDocument();
      if (doc) applyToDocument(doc, preferences);
    },
    getEnvironment() {
      const win = currentWindow();
      return {
        systemAppearance: readSystemAppearance(win),
        reducedMotion:
          win?.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false,
        forcedColors:
          win?.matchMedia?.("(forced-colors: active)").matches ?? false,
        online: win?.navigator.onLine ?? true,
      };
    },
    subscribe(listener: AppearanceAdapterListener) {
      const win = currentWindow();
      if (!win) return () => undefined;

      const systemQuery = win.matchMedia?.("(prefers-color-scheme: dark)");
      const onSystemChange = (event: MediaQueryListEvent) => {
        listener({
          type: "system-appearance-changed",
          systemAppearance: event.matches ? "dark" : "light",
        });
      };
      const onStorage = (event: StorageEvent) => {
        const accountId = event.key
          ? accountIdFromStorageKey(event.key)
          : null;
        if (accountId) {
          listener({
            type: "external-preferences-changed",
            value:
              event.newValue === null ? null : parseStoredValue(event.newValue),
            accountId,
          });
          return;
        }
        if (
          event.key !== BROWSER_APPEARANCE_STORAGE_KEY &&
          event.key !== LEGACY_SKIN_STORAGE_KEY &&
          event.key !== LEGACY_APPEARANCE_STORAGE_KEY
        ) {
          return;
        }
        // Account-scoped writes already emitted a canonical storage event.
        // Ignore their compatibility projection so another signed-in account
        // cannot import or upload it from a separate tab.
        try {
          if (
            source !== "docs" &&
            win.localStorage.getItem(BROWSER_APPEARANCE_OWNER_STORAGE_KEY) !==
              null
          ) {
            return;
          }
          listener({
            type: "external-preferences-changed",
            value: safeRead(win),
          });
        } catch {
          listener({ type: "storage-error" });
        }
      };
      const onOnline = () =>
        listener({ type: "connectivity-changed", online: true });
      const onOffline = () =>
        listener({ type: "connectivity-changed", online: false });
      const refreshVisibleAccount = () => {
        if (win.document.visibilityState === "hidden") return;
        listener({
          type: "connectivity-changed",
          online: win.navigator.onLine,
        });
      };

      systemQuery?.addEventListener("change", onSystemChange);
      win.addEventListener("storage", onStorage);
      win.addEventListener("online", onOnline);
      win.addEventListener("offline", onOffline);
      win.addEventListener("focus", refreshVisibleAccount);
      win.document.addEventListener("visibilitychange", refreshVisibleAccount);
      return () => {
        systemQuery?.removeEventListener("change", onSystemChange);
        win.removeEventListener("storage", onStorage);
        win.removeEventListener("online", onOnline);
        win.removeEventListener("offline", onOffline);
        win.removeEventListener("focus", refreshVisibleAccount);
        win.document.removeEventListener(
          "visibilitychange",
          refreshVisibleAccount,
        );
      };
    },
  };
}

/** Apply the versioned cache synchronously before React creates the root. */
export function applyCachedBrowserAppearanceBeforePaint(): void {
  const win = currentWindow();
  const doc = currentDocument();
  if (!win || !doc) return;
  const environment = readSystemAppearance(win);
  let raw: unknown | null = null;
  try {
    raw = readBootstrapValue(win);
  } catch {
    // Hardened storage must not prevent the application root from mounting.
  }
  const preferences = hasFutureAppearanceContractVersion(raw)
    ? createDefaultAppearancePreferences(environment)
    : parseAppearancePreferences(raw, {
        systemAppearance: environment,
      }).preferences;
  applyToDocument(doc, preferences);
}
