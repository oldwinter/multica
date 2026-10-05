import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  hasFutureAppearanceContractVersion,
  resolveAppearance,
  type AppearanceAdapterEvent,
  type AppearanceAdapterListener,
  type AppearanceEnvironment,
  type AppearancePreferenceAdapter,
  type AppearancePreferences,
  type Awaitable,
  type RequestedAppearance,
  type ResolvedAppearance,
  type SkinId,
} from "@multica/core/appearance";

export const APPEARANCE_STORAGE_KEY = "appearance-preferences";
export const APPEARANCE_OWNER_STORAGE_KEY = "appearance-preferences-owner";
export const THEME_STORAGE_KEY = "theme-preference";
export const SKIN_STORAGE_KEY = "skin-preference";

export type ThemePreference = RequestedAppearance;

export interface MobileAppearanceStorage {
  readItem(key: string): string | null;
  writeItem(key: string, value: string): Awaitable<void>;
}

export interface MobileAppearancePlatform {
  apply(preferences: AppearancePreferences): Awaitable<void>;
  getSystemAppearance(): Awaitable<ResolvedAppearance>;
  getReducedMotion(): Awaitable<boolean>;
  getForcedColors(): Awaitable<boolean>;
  getOnline(): Awaitable<boolean>;
  subscribeSystemAppearance?(
    listener: (appearance: ResolvedAppearance) => void,
  ): () => void;
  subscribeConnectivity?(listener: (online: boolean) => void): () => void;
}

export interface MobileAppearanceAdapterDependencies {
  storage: MobileAppearanceStorage;
  platform: MobileAppearancePlatform;
}

export function appearanceAccountStorageKey(userId: string): string {
  let encodedUserId = "";
  for (let index = 0; index < userId.length; index += 1) {
    encodedUserId += userId.charCodeAt(index).toString(16).padStart(4, "0");
  }
  return `${APPEARANCE_STORAGE_KEY}.user.${encodedUserId}`;
}

export function selectAccountAppearanceValue(
  accountValue: unknown | null,
  bootstrapValue: unknown | null,
  bootstrapOwner: string | null,
): unknown | null {
  if (accountValue !== null) return accountValue;
  return bootstrapOwner === null ? bootstrapValue : null;
}

export function hasFutureAppearanceTokenVersion(value: unknown): boolean {
  return hasFutureAppearanceContractVersion(value);
}

export async function persistMobileAppearanceCache(options: {
  storage: MobileAppearanceStorage;
  preferences: AppearancePreferences;
  accountId: string | null;
  writable: boolean;
  persistBootstrap(): Awaitable<void>;
}): Promise<boolean> {
  if (!options.writable) return false;
  if (options.accountId) {
    await options.storage.writeItem(
      appearanceAccountStorageKey(options.accountId),
      JSON.stringify(options.preferences),
    );
    await options.storage.writeItem(
      APPEARANCE_OWNER_STORAGE_KEY,
      options.accountId,
    );
  }
  await options.persistBootstrap();
  return true;
}

function isSkin(value: string | null): value is SkinId {
  return value === "tension" || value === "relay" || value === "field";
}

function isAppearance(value: string | null): value is RequestedAppearance {
  return value === "system" || value === "light" || value === "dark";
}

function legacyPreference(
  storedSkin: string | null,
  storedAppearance: string | null,
  systemAppearance: ResolvedAppearance,
): unknown | null {
  if (storedSkin === null && storedAppearance === null) return null;

  const skin = isSkin(storedSkin) ? storedSkin : storedSkin ?? "tension";
  const requestedAppearance = isAppearance(storedAppearance)
    ? storedAppearance
    : storedAppearance ?? "system";
  const resolvableAppearance = isAppearance(requestedAppearance)
    ? requestedAppearance
    : "system";

  return {
    version: APPEARANCE_PREFERENCES_VERSION,
    tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    skin,
    requestedAppearance,
    resolvedAppearance: resolveAppearance(
      resolvableAppearance,
      systemAppearance,
    ),
    source: "local",
    updatedAt: "1970-01-01T00:00:00.000Z",
    syncState: { status: "pending" },
  };
}

/**
 * SecureStore-backed mobile adapter. The full value is canonical; the two
 * legacy keys remain write-through during the installed-client migration.
 */
export function createMobileAppearanceAdapter({
  storage,
  platform,
}: MobileAppearanceAdapterDependencies): AppearancePreferenceAdapter {
  const listeners = new Set<AppearanceAdapterListener>();
  const unsubscribers = new Set<() => void>();

  const emit = (event: AppearanceAdapterEvent) => {
    for (const listener of listeners) listener(event);
  };

  return {
    source: "mobile",
    supportsRemoteSync: true,
    load: () => {
      const serialized = storage.readItem(APPEARANCE_STORAGE_KEY);
      if (serialized !== null) {
        try {
          return JSON.parse(serialized) as unknown;
        } catch {
          return serialized;
        }
      }

      const systemAppearance = platform.getSystemAppearance();
      if (systemAppearance instanceof Promise) {
        return systemAppearance.then((resolved) =>
          legacyPreference(
            storage.readItem(SKIN_STORAGE_KEY),
            storage.readItem(THEME_STORAGE_KEY),
            resolved,
          ),
        );
      }
      return legacyPreference(
        storage.readItem(SKIN_STORAGE_KEY),
        storage.readItem(THEME_STORAGE_KEY),
        systemAppearance,
      );
    },
    persist: async (preferences) => {
      await Promise.all([
        storage.writeItem(APPEARANCE_STORAGE_KEY, JSON.stringify(preferences)),
        storage.writeItem(SKIN_STORAGE_KEY, preferences.skin),
        storage.writeItem(THEME_STORAGE_KEY, preferences.requestedAppearance),
      ]);
    },
    apply: (preferences) => platform.apply(preferences),
    getEnvironment: async (): Promise<AppearanceEnvironment> => {
      const [systemAppearance, reducedMotion, forcedColors, online] =
        await Promise.all([
          platform.getSystemAppearance(),
          platform.getReducedMotion(),
          platform.getForcedColors(),
          platform.getOnline(),
        ]);
      return { systemAppearance, reducedMotion, forcedColors, online };
    },
    subscribe: (listener) => {
      listeners.add(listener);
      if (listeners.size === 1) {
        if (platform.subscribeSystemAppearance) {
          unsubscribers.add(
            platform.subscribeSystemAppearance((systemAppearance) => {
              emit({ type: "system-appearance-changed", systemAppearance });
            }),
          );
        }
        if (platform.subscribeConnectivity) {
          unsubscribers.add(
            platform.subscribeConnectivity((online) => {
              emit({ type: "connectivity-changed", online });
            }),
          );
        }
      }

      return () => {
        listeners.delete(listener);
        if (listeners.size > 0) return;
        for (const unsubscribe of unsubscribers) unsubscribe();
        unsubscribers.clear();
      };
    },
  };
}
