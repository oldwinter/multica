import {
  colorScheme,
  useColorScheme as useNativewindColorScheme,
} from "nativewind";
import { useCallback, useEffect, useMemo } from "react";
import * as SecureStore from "expo-secure-store";
import NetInfo from "@react-native-community/netinfo";
import { AccessibilityInfo, Appearance, AppState } from "react-native";
import { create } from "zustand";
import {
  changeAppearancePreferences,
  createAppearanceDiagnostics,
  markAppearanceSyncFailed,
  nextAppearancePreferenceTimestamp,
  parseAppearancePreferences,
  resetAppearancePreferences,
  withResolvedAppearance,
  type AppearanceEnvironment,
  type AppearancePreferenceField,
  type AppearancePreferences,
  type RequestedAppearance,
  type ResolvedAppearance,
  type SkinId,
} from "@multica/core/appearance";
import { useAuthStore } from "@/data/auth-store";
import {
  captureMobileAppearanceEvent,
  retryMobileAppearanceAnalytics,
} from "@/data/appearance-analytics";
import {
  NAV_THEMES,
  SKIN_IDS,
  THEMES,
  type AppColorScheme,
  type AppSkin,
} from "@/lib/theme";
import {
  createMobileAppearanceAdapter,
  APPEARANCE_OWNER_STORAGE_KEY,
  appearanceAccountStorageKey,
  hasFutureAppearanceTokenVersion,
  persistMobileAppearanceCache,
  selectAccountAppearanceValue,
  type ThemePreference,
} from "@/lib/appearance-preferences";
import { createAppearanceSyncCoordinator } from "@/lib/appearance-sync-coordinator";

export type { ThemePreference } from "@/lib/appearance-preferences";

function systemAppearance(): ResolvedAppearance {
  return Appearance.getColorScheme() === "dark" ? "dark" : "light";
}

const INITIAL_ENVIRONMENT: AppearanceEnvironment = {
  systemAppearance: systemAppearance(),
  reducedMotion: false,
  forcedColors: false,
  online: true,
};

const appearanceAdapter = createMobileAppearanceAdapter({
  storage: {
    readItem: (key) => SecureStore.getItem(key),
    writeItem: (key, value) => SecureStore.setItemAsync(key, value),
  },
  platform: {
    apply: (preferences) => {
      colorScheme.set(preferences.requestedAppearance);
    },
    getSystemAppearance: systemAppearance,
    getReducedMotion: () =>
      AccessibilityInfo.isReduceMotionEnabled().catch(() => false),
    getForcedColors: () =>
      AccessibilityInfo.isHighTextContrastEnabled().catch(() => false),
    getOnline: async () => (await NetInfo.fetch()).isConnected !== false,
    subscribeSystemAppearance: (listener) => {
      const subscription = Appearance.addChangeListener(
        ({ colorScheme: next }) => {
          listener(next === "dark" ? "dark" : "light");
        },
      );
      return () => subscription.remove();
    },
    subscribeConnectivity: (listener) =>
      NetInfo.addEventListener((state) => {
        if (state.isConnected !== null) listener(state.isConnected);
      }),
  },
});

let initialStorageFailed = false;
let initialValue: unknown | null = null;
let bootstrapOwner: string | null = null;
try {
  const loaded = appearanceAdapter.load();
  if (!(loaded instanceof Promise)) initialValue = loaded;
} catch {
  initialStorageFailed = true;
}
try {
  bootstrapOwner = SecureStore.getItem(APPEARANCE_OWNER_STORAGE_KEY);
} catch {
  initialStorageFailed = true;
}
let bootstrapValue = initialValue;
let localCacheWritable = !hasFutureAppearanceTokenVersion(initialValue);
const initialParsed = parseAppearancePreferences(initialValue, {
  systemAppearance: INITIAL_ENVIRONMENT.systemAppearance,
});
const initialPreferences = !localCacheWritable
  ? markAppearanceSyncFailed(initialParsed.preferences, "conflict")
  : initialStorageFailed
    ? markAppearanceSyncFailed(initialParsed.preferences, "storage")
    : initialParsed.preferences;
let initialPersistencePending =
  localCacheWritable &&
  (initialStorageFailed || initialParsed.recovered || initialValue !== null);
let initialTelemetryPending = true;
let lastSyncErrorClass =
  initialPreferences.syncState.status === "failed"
    ? initialPreferences.syncState.errorClass
    : null;

// SecureStore is read synchronously before the first component renders. This
// is the native pre-render equivalent of the Web bootstrap attribute script.
colorScheme.set(initialPreferences.requestedAppearance);

interface AppearanceState {
  preferences: AppearancePreferences;
  environment: AppearanceEnvironment;
  recoveredFields: readonly AppearancePreferenceField[];
  setPreferences(preferences: AppearancePreferences): void;
  setEnvironment(environment: AppearanceEnvironment): void;
  setRecoveredFields(fields: readonly AppearancePreferenceField[]): void;
  addRecoveredFields(fields: readonly AppearancePreferenceField[]): void;
}

const useAppearanceState = create<AppearanceState>((set) => ({
  preferences: initialPreferences,
  environment: INITIAL_ENVIRONMENT,
  recoveredFields: initialParsed.issues.map((issue) => issue.field),
  setPreferences: (preferences) => set({ preferences }),
  setEnvironment: (environment) => set({ environment }),
  setRecoveredFields: (recoveredFields) => set({ recoveredFields }),
  addRecoveredFields: (fields) =>
    set((state) => ({
      recoveredFields: [...new Set([...state.recoveredFields, ...fields])],
    })),
}));

function publishAppearancePreferences(
  preferences: AppearancePreferences,
): void {
  const previous = useAppearanceState.getState().preferences;
  useAppearanceState.getState().setPreferences(preferences);
  if (
    preferences.syncState.status === "failed" &&
    (previous.syncState.status !== "failed" ||
      previous.syncState.errorClass !== preferences.syncState.errorClass)
  ) {
    captureMobileAppearanceEvent("sync_failed", {
      adapterSource: "mobile",
      errorClass: preferences.syncState.errorClass,
    });
    lastSyncErrorClass = preferences.syncState.errorClass;
  }
}

function captureAppearanceRecoveryIfSettled(): void {
  if (!lastSyncErrorClass) return;
  const status = useAppearanceState.getState().preferences.syncState.status;
  if (
    status !== "synced" &&
    !(lastSyncErrorClass === "storage" && status === "local-only")
  ) {
    return;
  }
  captureMobileAppearanceEvent("sync_recovered", {
    adapterSource: "mobile",
    previousErrorClass: lastSyncErrorClass,
  });
  lastSyncErrorClass = null;
}

function addRecoveredAppearanceFields(
  fields: readonly AppearancePreferenceField[],
  captureRecovery = true,
): void {
  const existing = new Set(useAppearanceState.getState().recoveredFields);
  useAppearanceState.getState().addRecoveredFields(fields);
  if (!captureRecovery) return;
  for (const field of fields) {
    if (existing.has(field)) continue;
    captureMobileAppearanceEvent("invalid_value_recovered", {
      adapterSource: "mobile",
      field,
    });
    existing.add(field);
  }
}

async function commitAppearance(
  preferences: AppearancePreferences,
  persist = true,
): Promise<void> {
  const committed = localCacheWritable
    ? preferences
    : markAppearanceSyncFailed(preferences, "conflict");
  publishAppearancePreferences(committed);
  try {
    await appearanceAdapter.apply(committed);
  } catch {
    const latest = useAppearanceState.getState().preferences;
    if (latest.updatedAt === committed.updatedAt) {
      publishAppearancePreferences(markAppearanceSyncFailed(latest, "unknown"));
    }
  }
  if (!localCacheWritable) {
    return;
  }
  if (!persist) return;

  try {
    await queueAppearancePersistence(committed);
    captureAppearanceRecoveryIfSettled();
  } catch {
    const latest = useAppearanceState.getState().preferences;
    if (latest.updatedAt === committed.updatedAt) {
      publishAppearancePreferences(markAppearanceSyncFailed(latest, "storage"));
    }
  }
}

let persistenceQueue: Promise<void> = Promise.resolve();

function queueAppearancePersistence(
  preferences: AppearancePreferences,
): Promise<void> {
  const accountId = useAuthStore.getState().user?.id ?? null;
  const cacheWritable = localCacheWritable;
  persistenceQueue = persistenceQueue
    .catch(() => undefined)
    .then(async () => {
      const persisted = await persistMobileAppearanceCache({
        storage: {
          readItem: (key) => SecureStore.getItem(key),
          writeItem: (key, value) => SecureStore.setItemAsync(key, value),
        },
        preferences,
        accountId,
        writable: cacheWritable,
        persistBootstrap: () => appearanceAdapter.persist(preferences),
      });
      if (!persisted) return;
      if (accountId) bootstrapOwner = accountId;
      bootstrapValue = preferences;
    });
  return persistenceQueue;
}

function readAccountAppearanceValue(userId: string): unknown | null {
  const serialized = SecureStore.getItem(appearanceAccountStorageKey(userId));
  if (serialized === null) return null;
  try {
    return JSON.parse(serialized) as unknown;
  } catch {
    return serialized;
  }
}

async function activateAccountAppearance(
  userId: string,
  environment: AppearanceEnvironment,
): Promise<AppearancePreferences> {
  let accountValue: unknown | null = null;
  let storageFailed = false;
  try {
    accountValue = readAccountAppearanceValue(userId);
  } catch {
    storageFailed = true;
  }
  const selectedValue = selectAccountAppearanceValue(
    accountValue,
    bootstrapValue,
    bootstrapOwner,
  );
  localCacheWritable = !hasFutureAppearanceTokenVersion(selectedValue);
  const parsed = parseAppearancePreferences(selectedValue, {
    systemAppearance: environment.systemAppearance,
  });
  useAppearanceState.getState().setRecoveredFields([]);
  addRecoveredAppearanceFields(
    parsed.issues.map((issue) => issue.field),
    localCacheWritable,
  );
  const preferences = !localCacheWritable
    ? markAppearanceSyncFailed(parsed.preferences, "conflict")
    : storageFailed
      ? markAppearanceSyncFailed(parsed.preferences, "storage")
      : parsed.preferences;
  lastSyncErrorClass =
    preferences.syncState.status === "failed"
      ? preferences.syncState.errorClass
      : null;
  await commitAppearance(preferences, parsed.recovered || storageFailed);
  return useAppearanceState.getState().preferences;
}

async function refreshEnvironment(): Promise<AppearanceEnvironment> {
  try {
    const environment = await appearanceAdapter.getEnvironment();
    useAppearanceState.getState().setEnvironment(environment);
    return environment;
  } catch {
    return useAppearanceState.getState().environment;
  }
}

const appearanceSyncCoordinator = createAppearanceSyncCoordinator({
  getUser: () => useAuthStore.getState().user,
  getUserId: (user) =>
    typeof user === "object" &&
    user !== null &&
    "id" in user &&
    typeof user.id === "string"
      ? user.id
      : null,
  getPreferences: () => useAppearanceState.getState().preferences,
  isLocalCacheWritable: () => localCacheWritable,
  refreshEnvironment,
  prepareLocalForUser: activateAccountAppearance,
  switchToSystemAppearance: async () => {
    colorScheme.set("system");
    return refreshEnvironment();
  },
  commit: commitAppearance,
  addRecoveredFields: addRecoveredAppearanceFields,
  updateAppearance: (data) =>
    useAuthStore.getState().updateAppearancePreferences(data),
  refreshUser: () => useAuthStore.getState().refreshUser(),
});

const requestAppearanceSync = () => appearanceSyncCoordinator.requestSync();
const refreshAuthenticatedAppearance = () =>
  appearanceSyncCoordinator.refreshAuthenticated();

function applyExplicitChange(change: {
  skin?: SkinId;
  requestedAppearance?: RequestedAppearance;
}): void {
  const current = useAppearanceState.getState().preferences;
  const nextSkin = change.skin ?? current.skin;
  const nextAppearance =
    change.requestedAppearance ?? current.requestedAppearance;
  if (
    nextSkin === current.skin &&
    nextAppearance === current.requestedAppearance
  ) {
    return;
  }

  if (change.skin && change.skin !== current.skin) {
    captureMobileAppearanceEvent("skin_selected", {
      skin: change.skin,
      previousSkin: current.skin,
      adapterSource: "mobile",
    });
  }
  if (
    change.requestedAppearance &&
    change.requestedAppearance !== current.requestedAppearance
  ) {
    captureMobileAppearanceEvent("appearance_selected", {
      appearance: change.requestedAppearance,
      previousAppearance: current.requestedAppearance,
      adapterSource: "mobile",
    });
  }

  if (change.requestedAppearance === "system") colorScheme.set("system");
  const currentEnvironment = useAppearanceState.getState().environment;
  const liveSystemAppearance = systemAppearance();
  const environment = {
    ...currentEnvironment,
    systemAppearance: liveSystemAppearance,
  };
  useAppearanceState.getState().setEnvironment(environment);
  const changed = changeAppearancePreferences(current, change, {
    updatedAt: nextAppearancePreferenceTimestamp(current.updatedAt),
    systemAppearance: liveSystemAppearance,
  });
  void commitAppearance(changed).then(() => requestAppearanceSync());
}

function resetAppearance(): void {
  captureMobileAppearanceEvent("reset", { adapterSource: "mobile" });
  colorScheme.set("system");
  const liveSystemAppearance = systemAppearance();
  const current = useAppearanceState.getState().preferences;
  const reset = resetAppearancePreferences(
    nextAppearancePreferenceTimestamp(current.updatedAt),
    liveSystemAppearance,
  );
  useAppearanceState.getState().setEnvironment({
    ...useAppearanceState.getState().environment,
    systemAppearance: liveSystemAppearance,
  });
  void commitAppearance(reset).then(() => requestAppearanceSync());
}

function retryAppearanceSync(): void {
  const current = useAppearanceState.getState().preferences;
  const retrying =
    current.syncState.status === "failed" &&
    current.syncState.errorClass === "storage"
      ? {
          ...current,
          syncState:
            current.source === "server"
              ? ({ status: "synced" } as const)
              : current.source === "local"
                ? ({ status: "pending" } as const)
                : ({ status: "local-only" } as const),
        }
      : current;
  void retryMobileAppearanceAnalytics();
  void commitAppearance(retrying).then(() => refreshAuthenticatedAppearance());
}

export function recordMobileAppearanceViewed(): void {
  const preferences = useAppearanceState.getState().preferences;
  captureMobileAppearanceEvent("appearance_viewed", {
    skin: preferences.skin,
    requestedAppearance: preferences.requestedAppearance,
    resolvedAppearance: preferences.resolvedAppearance,
    adapterSource: "mobile",
  });
}

export function useAppearanceSync(): void {
  const user = useAuthStore((state) => state.user);

  useEffect(() => {
    void appearanceSyncCoordinator.reconcileUser(user);
  }, [user]);

  useEffect(() => {
    if (initialTelemetryPending) {
      initialTelemetryPending = false;
      if (localCacheWritable) {
        for (const issue of initialParsed.issues) {
          captureMobileAppearanceEvent("invalid_value_recovered", {
            adapterSource: "mobile",
            field: issue.field,
          });
        }
      }
      if (initialPreferences.syncState.status === "failed") {
        captureMobileAppearanceEvent("sync_failed", {
          adapterSource: "mobile",
          errorClass: initialPreferences.syncState.errorClass,
        });
      }
    }
    void refreshEnvironment().then((environment) => {
      const current = useAppearanceState.getState().preferences;
      let resolved = withResolvedAppearance(
        current,
        environment.systemAppearance,
      );
      if (initialPersistencePending) {
        initialPersistencePending = false;
        if (initialStorageFailed) {
          resolved = { ...resolved, syncState: { status: "local-only" } };
        }
        void commitAppearance(resolved);
      } else if (resolved !== current) {
        void commitAppearance(resolved);
      }
    });

    const unsubscribe = appearanceAdapter.subscribe((event) => {
      if (event.type === "system-appearance-changed") {
        const state = useAppearanceState.getState();
        state.setEnvironment({
          ...state.environment,
          systemAppearance: event.systemAppearance,
        });
        const resolved = withResolvedAppearance(
          state.preferences,
          event.systemAppearance,
        );
        if (resolved !== state.preferences) void commitAppearance(resolved);
        return;
      }
      if (event.type === "connectivity-changed") {
        const state = useAppearanceState.getState();
        state.setEnvironment({ ...state.environment, online: event.online });
        if (event.online) {
          void retryMobileAppearanceAnalytics();
          void refreshAuthenticatedAppearance();
        }
        return;
      }
      if (event.type === "storage-error") {
        localCacheWritable = false;
        publishAppearancePreferences(
          markAppearanceSyncFailed(
            useAppearanceState.getState().preferences,
            "storage",
          ),
        );
        return;
      }
      localCacheWritable = !hasFutureAppearanceTokenVersion(event.value);
      const parsed = parseAppearancePreferences(event.value, {
        systemAppearance: useAppearanceState.getState().environment
          .systemAppearance,
      });
      addRecoveredAppearanceFields(
        parsed.issues.map((issue) => issue.field),
        localCacheWritable,
      );
      void commitAppearance(parsed.preferences);
    });

    const reduceMotionSubscription = AccessibilityInfo.addEventListener(
      "reduceMotionChanged",
      (reducedMotion) => {
        const state = useAppearanceState.getState();
        state.setEnvironment({ ...state.environment, reducedMotion });
      },
    );
    const contrastSubscription = AccessibilityInfo.addEventListener(
      "highTextContrastChanged",
      (forcedColors) => {
        const state = useAppearanceState.getState();
        state.setEnvironment({ ...state.environment, forcedColors });
      },
    );
    const appStateSubscription = AppState.addEventListener(
      "change",
      (status) => {
        if (status === "active") void refreshAuthenticatedAppearance();
      },
    );

    return () => {
      unsubscribe();
      reduceMotionSubscription.remove();
      contrastSubscription.remove();
      appStateSubscription.remove();
    };
  }, []);
}

export async function getMobileAppearanceDiagnostics() {
  const environment = await refreshEnvironment();
  const state = useAppearanceState.getState();
  return createAppearanceDiagnostics(state.preferences, {
    adapterSource: "mobile",
    reducedMotion: environment.reducedMotion,
    forcedColors: environment.forcedColors,
    recoveredFields: state.recoveredFields,
  });
}

export function useColorScheme() {
  const { colorScheme: nativeColorScheme } = useNativewindColorScheme();
  const preferences = useAppearanceState((state) => state.preferences);
  const environment = useAppearanceState((state) => state.environment);
  const recoveredFields = useAppearanceState((state) => state.recoveredFields);
  const setPreference = useCallback(
    (next: ThemePreference) =>
      applyExplicitChange({ requestedAppearance: next }),
    [],
  );
  const setSkin = useCallback(
    (next: AppSkin) => applyExplicitChange({ skin: next }),
    [],
  );
  const retrySync = useCallback(() => {
    retryAppearanceSync();
  }, []);
  const reset = useCallback(() => resetAppearance(), []);

  const resolvedScheme: AppColorScheme =
    nativeColorScheme === "dark" ? "dark" : "light";
  const diagnostics = useMemo(
    () =>
      createAppearanceDiagnostics(preferences, {
        adapterSource: "mobile",
        reducedMotion: environment.reducedMotion,
        forcedColors: environment.forcedColors,
        recoveredFields,
      }),
    [
      environment.forcedColors,
      environment.reducedMotion,
      preferences,
      recoveredFields,
    ],
  );

  return {
    colorScheme: resolvedScheme,
    preference: preferences.requestedAppearance,
    preferences,
    setPreference,
    skin: preferences.skin,
    setSkin,
    skins: SKIN_IDS,
    theme: THEMES[preferences.skin][resolvedScheme],
    navigationTheme: NAV_THEMES[preferences.skin][resolvedScheme],
    isDarkColorScheme: resolvedScheme === "dark",
    online: environment.online,
    diagnostics,
    retrySync,
    reset,
  };
}
