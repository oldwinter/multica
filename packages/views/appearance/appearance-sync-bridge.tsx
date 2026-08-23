"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  changeAppearancePreferences,
  createAppearanceDiagnostics,
  createDefaultAppearancePreferences,
  hasFutureAppearanceContractVersion,
  markAppearanceSyncFailed,
  markAppearanceSynced,
  nextAppearancePreferenceTimestamp,
  parseAppearancePreferences,
  reconcileAppearancePreferences,
  resetAppearancePreferences,
  resolveAppearance,
  withResolvedAppearance,
  type AppearanceAnalyticsCapture,
  type AppearanceDiagnosticsSnapshot,
  type AppearanceEnvironment,
  type AppearancePreferenceAdapter,
  type AppearancePreferences,
  type AppearanceSyncErrorClass,
  type RequestedAppearance,
  type ResolvedAppearance,
  type SkinId,
} from "@multica/core/appearance";
import { captureEvent } from "@multica/core/analytics";
import type { UpdateMeRequest, User } from "@multica/core/types";
import {
  useSkin,
  useTheme,
} from "@multica/ui/components/common/theme-provider";

type ServerAppearance = {
  preferences: AppearancePreferences | null;
  writable: boolean;
};

type AppearanceSyncContextValue = {
  preferences: AppearancePreferences;
  diagnostics: AppearanceDiagnosticsSnapshot;
  isReady: boolean;
  selectSkin: (skin: SkinId) => void;
  selectAppearance: (appearance: RequestedAppearance) => void;
  reset: () => void;
  retry: () => void;
};

type AppearancePersistScope = "bootstrap" | "account";
type AppearanceAccountUpdate = Required<
  Pick<
    UpdateMeRequest,
    "skin" | "appearance" | "appearanceUpdatedAt" | "appearanceTokenVersion"
  >
>;

const AppearanceSyncContext = createContext<AppearanceSyncContextValue | null>(
  null,
);

const DEFAULT_ENVIRONMENT: AppearanceEnvironment = {
  systemAppearance: "light",
  reducedMotion: false,
  forcedColors: false,
  online: true,
};

function serverAppearanceFromUser(
  user: User,
  systemAppearance: ResolvedAppearance,
): ServerAppearance {
  const values = [
    user.skin,
    user.appearance,
    user.appearanceUpdatedAt,
    user.appearanceTokenVersion,
  ];
  if (values.every((value) => value === null)) {
    return { preferences: null, writable: true };
  }
  if (
    values.some((value) => value === null) ||
    user.appearanceTokenVersion !== APPEARANCE_TOKEN_CONTRACT_VERSION
  ) {
    return { preferences: null, writable: false };
  }

  const requestedAppearance = user.appearance!;
  const parsed = parseAppearancePreferences(
    {
      version: APPEARANCE_PREFERENCES_VERSION,
      tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
      skin: user.skin,
      requestedAppearance,
      resolvedAppearance: resolveAppearance(
        requestedAppearance,
        systemAppearance,
      ),
      source: "server",
      updatedAt: user.appearanceUpdatedAt,
      syncState: { status: "synced" },
    },
    { systemAppearance },
  );
  return parsed.recovered
    ? { preferences: null, writable: false }
    : { preferences: parsed.preferences, writable: true };
}

function classifySyncError(error: unknown): AppearanceSyncErrorClass {
  const status =
    typeof error === "object" &&
    error !== null &&
    "status" in error &&
    typeof error.status === "number"
      ? error.status
      : null;
  if (status !== null) {
    if (status === 401 || status === 403) return "unauthorized";
    if (status === 409) return "conflict";
    if (status >= 500) return "server";
  }
  if (
    error instanceof TypeError ||
    (typeof navigator !== "undefined" && navigator.onLine === false)
  ) {
    return "network";
  }
  return "unknown";
}

function sameExplicitPreference(
  first: AppearancePreferences,
  second: AppearancePreferences,
): boolean {
  return (
    first.updatedAt === second.updatedAt &&
    first.skin === second.skin &&
    first.requestedAppearance === second.requestedAppearance &&
    first.tokenContractVersion === second.tokenContractVersion
  );
}

function appearanceSyncKey(
  accountId: string,
  preferences: AppearancePreferences,
): string {
  return [
    accountId,
    preferences.updatedAt,
    preferences.skin,
    preferences.requestedAppearance,
    preferences.tokenContractVersion,
  ].join(":");
}

const defaultCapture: AppearanceAnalyticsCapture = (name, properties) => {
  captureEvent(name, properties);
};

export function AppearanceSyncBridge({
  adapter,
  account = null,
  updateAccountAppearance,
  refreshAccountAppearance,
  capture = defaultCapture,
  children,
}: {
  adapter: AppearancePreferenceAdapter;
  account?: User | null;
  updateAccountAppearance?: (data: AppearanceAccountUpdate) => Promise<User>;
  refreshAccountAppearance?: () => Promise<User>;
  capture?: AppearanceAnalyticsCapture;
  children: ReactNode;
}) {
  const { setSkin } = useSkin();
  const { setTheme } = useTheme();
  const setSkinRef = useRef(setSkin);
  const setThemeRef = useRef(setTheme);
  setSkinRef.current = setSkin;
  setThemeRef.current = setTheme;
  const user = account;
  const userRef = useRef(user);
  userRef.current = user;

  const [environment, setEnvironment] =
    useState<AppearanceEnvironment>(DEFAULT_ENVIRONMENT);
  const [preferences, setPreferences] = useState<AppearancePreferences>(() =>
    createDefaultAppearancePreferences("light"),
  );
  const [isReady, setIsReady] = useState(false);
  const preferencesRef = useRef(preferences);
  const environmentRef = useRef(environment);
  const remoteWritableRef = useRef(true);
  const localCacheWritableRef = useRef(true);
  const syncingRequests = useRef(new Set<string>());
  const appearanceOperationSequence = useRef(0);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const publish = useCallback((next: AppearancePreferences) => {
    preferencesRef.current = next;
    if (mountedRef.current) setPreferences(next);
  }, []);

  const apply = useCallback(
    async (
      next: AppearancePreferences,
      options: {
        animate: boolean;
        persist: boolean;
        persistScope?: AppearancePersistScope;
        accountId?: string;
      },
    ) => {
      if (options.accountId && userRef.current?.id !== options.accountId) {
        return false;
      }
      publish(next);
      if (options.animate) {
        setSkinRef.current(next.skin);
        setThemeRef.current(next.requestedAppearance);
        await adapter.apply(next);
      } else {
        await adapter.apply(next);
        setSkinRef.current(next.skin, { animate: false });
        setThemeRef.current(next.requestedAppearance, { animate: false });
      }
      if (options.accountId && userRef.current?.id !== options.accountId) {
        return false;
      }
      if (!options.persist || !localCacheWritableRef.current) return true;

      try {
        const accountId = options.accountId ?? userRef.current?.id;
        if (
          options.persistScope !== "bootstrap" &&
          accountId &&
          adapter.persistForAccount
        ) {
          await adapter.persistForAccount(accountId, next);
        } else {
          await adapter.persist(next);
        }
        return true;
      } catch {
        const failed = markAppearanceSyncFailed(next, "storage");
        if (sameExplicitPreference(preferencesRef.current, next)) {
          publish(failed);
        }
        capture("sync_failed", {
          adapterSource: adapter.source,
          errorClass: "storage",
        });
        return false;
      }
    },
    [adapter, capture, publish],
  );

  const syncToServer = useCallback(
    async (submitted: AppearancePreferences) => {
      const currentUser = userRef.current;
      if (
        !currentUser ||
        !adapter.supportsRemoteSync ||
        !updateAccountAppearance ||
        !remoteWritableRef.current ||
        !localCacheWritableRef.current
      ) {
        return;
      }

      const accountId = currentUser.id;
      const requestKey = appearanceSyncKey(accountId, submitted);
      if (syncingRequests.current.has(requestKey)) return;

      syncingRequests.current.add(requestKey);
      try {
        const updatedUser = await updateAccountAppearance({
          skin: submitted.skin,
          appearance: submitted.requestedAppearance,
          appearanceUpdatedAt: submitted.updatedAt,
          appearanceTokenVersion: submitted.tokenContractVersion,
        });
        if (
          userRef.current?.id !== accountId ||
          updatedUser.id !== accountId
        ) {
          return;
        }
        appearanceOperationSequence.current += 1;

        // A newer local click owns the screen; its own request will settle it.
        if (!sameExplicitPreference(preferencesRef.current, submitted)) return;
        const remote = serverAppearanceFromUser(
          updatedUser,
          environmentRef.current.systemAppearance,
        );
        if (!remote.preferences) {
          remoteWritableRef.current = remote.writable;
          return;
        }
        const settled = reconcileAppearancePreferences(
          submitted,
          remote.preferences,
          environmentRef.current.systemAppearance,
        ).preferences;
        const previousFailure =
          submitted.syncState.status === "failed"
            ? submitted.syncState.errorClass
            : null;
        const persisted = await apply(markAppearanceSynced(settled), {
          animate: false,
          persist: true,
          accountId,
        });
        if (previousFailure && persisted) {
          capture("sync_recovered", {
            adapterSource: adapter.source,
            previousErrorClass: previousFailure,
          });
        }
      } catch (error) {
        if (
          userRef.current?.id !== accountId ||
          !sameExplicitPreference(preferencesRef.current, submitted)
        ) {
          return;
        }
        const errorClass = classifySyncError(error);
        const failed = markAppearanceSyncFailed(submitted, errorClass);
        await apply(failed, {
          animate: false,
          persist: true,
          accountId,
        });
        capture("sync_failed", {
          adapterSource: adapter.source,
          errorClass,
        });
      } finally {
        syncingRequests.current.delete(requestKey);
      }
    },
    [adapter, apply, capture, updateAccountAppearance],
  );

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      let nextEnvironment = DEFAULT_ENVIRONMENT;
      let raw: unknown | null = null;
      let storageFailed = false;
      try {
        nextEnvironment = await adapter.getEnvironment();
      } catch {
        // Environment probes are enhancements; defaults keep startup usable.
      }
      const bootAccountId = userRef.current?.id;
      try {
        raw =
          bootAccountId && adapter.loadForAccount
            ? await adapter.loadForAccount(bootAccountId)
            : await adapter.load();
      } catch {
        storageFailed = true;
      }
      if (cancelled) return;

      environmentRef.current = nextEnvironment;
      setEnvironment(nextEnvironment);
      const futureContract = hasFutureAppearanceContractVersion(raw);
      localCacheWritableRef.current = !futureContract;
      const parsed = parseAppearancePreferences(raw, {
        systemAppearance: nextEnvironment.systemAppearance,
      });
      let bootPreferences =
        (!userRef.current || !adapter.supportsRemoteSync) &&
        parsed.preferences.source === "local"
          ? {
              ...parsed.preferences,
              syncState: { status: "local-only" as const },
            }
          : parsed.preferences;
      if (futureContract) {
        bootPreferences = markAppearanceSyncFailed(
          createDefaultAppearancePreferences(nextEnvironment.systemAppearance),
          "conflict",
        );
      } else if (storageFailed) {
        bootPreferences = markAppearanceSyncFailed(bootPreferences, "storage");
      }
      await apply(bootPreferences, {
        animate: false,
        persist: parsed.recovered && !futureContract && !storageFailed,
        persistScope: bootAccountId ? "account" : "bootstrap",
        accountId: bootAccountId,
      });
      if (cancelled) return;
      setIsReady(true);
      for (const issue of parsed.issues) {
        capture("invalid_value_recovered", {
          adapterSource: adapter.source,
          field: issue.field,
        });
      }
      if (storageFailed || futureContract) {
        capture("sync_failed", {
          adapterSource: adapter.source,
          errorClass: storageFailed ? "storage" : "conflict",
        });
      }
      capture("appearance_viewed", {
        skin: bootPreferences.skin,
        requestedAppearance: bootPreferences.requestedAppearance,
        resolvedAppearance: bootPreferences.resolvedAppearance,
        adapterSource: adapter.source,
      });
    })();
    return () => {
      cancelled = true;
    };
  }, [adapter, apply, capture]);

  const userAppearanceKey = user
    ? [
        user.id,
        user.skin,
        user.appearance,
        user.appearanceUpdatedAt,
        user.appearanceTokenVersion,
      ].join(":")
    : "anonymous";

  useEffect(() => {
    if (!isReady) return;
    if (!user) {
      remoteWritableRef.current = true;
      return;
    }

    let cancelled = false;
    const accountId = user.id;
    void (async () => {
      let raw: unknown | null = preferencesRef.current;
      let storageFailed = false;
      if (adapter.loadForAccount) {
        try {
          raw = await adapter.loadForAccount(accountId);
        } catch {
          raw = null;
          storageFailed = true;
        }
      }
      if (cancelled || userRef.current?.id !== accountId) return;

      const futureContract = hasFutureAppearanceContractVersion(raw);
      localCacheWritableRef.current = !futureContract;
      const parsed = parseAppearancePreferences(raw, {
        systemAppearance: environmentRef.current.systemAppearance,
      });
      const remote = serverAppearanceFromUser(
        user,
        environmentRef.current.systemAppearance,
      );
      remoteWritableRef.current = remote.writable;

      const reconciled = reconcileAppearancePreferences(
        futureContract ? null : parsed.preferences,
        remote.preferences,
        environmentRef.current.systemAppearance,
      );
      let next = reconciled.preferences;
      if (!remote.writable) {
        next = { ...next, syncState: { status: "local-only" as const } };
      }
      if (futureContract) {
        next = markAppearanceSyncFailed(next, "conflict");
      } else if (storageFailed) {
        next = markAppearanceSyncFailed(next, "storage");
      }

      await apply(next, {
        animate: false,
        persist:
          remote.writable &&
          (reconciled.shouldPersistLocal ||
            reconciled.winner === "default" ||
            (parsed.preferences.source === "default" &&
              remote.preferences === null)) &&
          !futureContract &&
          !storageFailed,
        accountId,
      });
      if (cancelled || userRef.current?.id !== accountId) return;

      for (const issue of parsed.issues) {
        capture("invalid_value_recovered", {
          adapterSource: adapter.source,
          field: issue.field,
        });
      }
      if (storageFailed || futureContract) {
        capture("sync_failed", {
          adapterSource: adapter.source,
          errorClass: storageFailed ? "storage" : "conflict",
        });
      }
      if (
        remote.writable &&
        !futureContract &&
        !storageFailed &&
        reconciled.shouldSyncServer
      ) {
        void syncToServer(reconciled.preferences);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [adapter, apply, capture, isReady, syncToServer, user, userAppearanceKey]);

  useEffect(() => {
    return adapter.subscribe((event) => {
      if (event.type === "system-appearance-changed") {
        const nextEnvironment = {
          ...environmentRef.current,
          systemAppearance: event.systemAppearance,
        };
        environmentRef.current = nextEnvironment;
        setEnvironment(nextEnvironment);
        const resolved = withResolvedAppearance(
          preferencesRef.current,
          event.systemAppearance,
        );
        void apply(resolved, {
          animate: false,
          persist: true,
          accountId: userRef.current?.id,
        });
        return;
      }
      if (event.type === "external-preferences-changed") {
        if (event.accountId && userRef.current?.id !== event.accountId) {
          return;
        }
        const futureContract = hasFutureAppearanceContractVersion(event.value);
        localCacheWritableRef.current = !futureContract;
        const parsed = parseAppearancePreferences(event.value, {
          systemAppearance: environmentRef.current.systemAppearance,
        });
        const remote = userRef.current
          ? serverAppearanceFromUser(
              userRef.current,
              environmentRef.current.systemAppearance,
            ).preferences
          : null;
        const reconciled = reconcileAppearancePreferences(
          futureContract ? null : parsed.preferences,
          remote,
          environmentRef.current.systemAppearance,
        );
        const next = futureContract
          ? markAppearanceSyncFailed(reconciled.preferences, "conflict")
          : reconciled.preferences;
        void apply(next, {
          animate: false,
          persist:
            !futureContract &&
            (reconciled.shouldPersistLocal || parsed.recovered),
          accountId: event.accountId ?? userRef.current?.id,
        }).then(() => {
          if (!futureContract && reconciled.shouldSyncServer) {
            void syncToServer(reconciled.preferences);
          }
        });
        if (futureContract) {
          capture("sync_failed", {
            adapterSource: adapter.source,
            errorClass: "conflict",
          });
        }
        return;
      }
      if (event.type === "storage-error") {
        const failed = markAppearanceSyncFailed(
          preferencesRef.current,
          "storage",
        );
        publish(failed);
        capture("sync_failed", {
          adapterSource: adapter.source,
          errorClass: "storage",
        });
        return;
      }
      const nextEnvironment = {
        ...environmentRef.current,
        online: event.online,
      };
      environmentRef.current = nextEnvironment;
      setEnvironment(nextEnvironment);
      if (event.online) {
        const accountId = userRef.current?.id;
        if (!accountId) return;
        const operationSequence = appearanceOperationSequence.current;
        if (!refreshAccountAppearance) return;
        void refreshAccountAppearance()
          .then((updatedUser) => {
            if (
              userRef.current?.id !== accountId ||
              updatedUser.id !== accountId ||
              appearanceOperationSequence.current !== operationSequence ||
              syncingRequests.current.size > 0
            ) {
              return;
            }
          })
          .catch(() => undefined)
          .finally(() => {
            if (userRef.current?.id !== accountId) return;
            const current = preferencesRef.current;
            if (
              current.syncState.status === "pending" ||
              current.syncState.status === "failed"
            ) {
              void syncToServer(current);
            }
          });
      }
    });
  }, [
    adapter,
    apply,
    capture,
    publish,
    refreshAccountAppearance,
    syncToServer,
  ]);

  const commitChange = useCallback(
    (change: { skin?: SkinId; requestedAppearance?: RequestedAppearance }) => {
      const current = preferencesRef.current;
      const next = changeAppearancePreferences(current, change, {
        updatedAt: nextAppearancePreferenceTimestamp(current.updatedAt),
        systemAppearance: environmentRef.current.systemAppearance,
      });
      if (next === current) return;
      appearanceOperationSequence.current += 1;
      const local =
        !userRef.current ||
        !adapter.supportsRemoteSync ||
        !remoteWritableRef.current ||
        !localCacheWritableRef.current
          ? { ...next, syncState: { status: "local-only" as const } }
          : next;
      const accountId = userRef.current?.id;
      void apply(local, {
        animate: true,
        persist: true,
        accountId,
      }).then(() => {
        void syncToServer(local);
      });
    },
    [adapter.supportsRemoteSync, apply, syncToServer],
  );

  const selectSkin = useCallback(
    (skin: SkinId) => {
      const previousSkin = preferencesRef.current.skin;
      if (skin === previousSkin) return;
      commitChange({ skin });
      capture("skin_selected", {
        skin,
        previousSkin,
        adapterSource: adapter.source,
      });
    },
    [adapter.source, capture, commitChange],
  );

  const selectAppearance = useCallback(
    (appearance: RequestedAppearance) => {
      const previousAppearance = preferencesRef.current.requestedAppearance;
      if (appearance === previousAppearance) return;
      commitChange({ requestedAppearance: appearance });
      capture("appearance_selected", {
        appearance,
        previousAppearance,
        adapterSource: adapter.source,
      });
    },
    [adapter.source, capture, commitChange],
  );

  const reset = useCallback(() => {
    const current = preferencesRef.current;
    const resetPreferences = resetAppearancePreferences(
      nextAppearancePreferenceTimestamp(current.updatedAt),
      environmentRef.current.systemAppearance,
    );
    appearanceOperationSequence.current += 1;
    const next =
      !userRef.current ||
      !adapter.supportsRemoteSync ||
      !remoteWritableRef.current ||
      !localCacheWritableRef.current
        ? {
            ...resetPreferences,
            syncState: { status: "local-only" as const },
          }
        : resetPreferences;
    const accountId = userRef.current?.id;
    void apply(next, {
      animate: true,
      persist: true,
      accountId,
    }).then(() => {
      void syncToServer(next);
    });
    capture("reset", { adapterSource: adapter.source });
  }, [
    adapter.source,
    adapter.supportsRemoteSync,
    apply,
    capture,
    syncToServer,
  ]);

  const retry = useCallback(() => {
    const current = preferencesRef.current;
    if (
      current.syncState.status === "failed" &&
      current.syncState.errorClass === "storage"
    ) {
      localCacheWritableRef.current = true;
      const retrying =
        current.source === "server"
          ? markAppearanceSynced(current)
          : current.source === "local"
            ? { ...current, syncState: { status: "pending" as const } }
            : { ...current, syncState: { status: "local-only" as const } };
      void apply(retrying, {
        animate: false,
        persist: true,
        accountId: userRef.current?.id,
      }).then(
        (persisted) => {
          if (persisted) {
            capture("sync_recovered", {
              adapterSource: adapter.source,
              previousErrorClass: "storage",
            });
            if (retrying.syncState.status === "pending") {
              void syncToServer(retrying);
            }
          }
        },
      );
      return;
    }
    void syncToServer(current);
  }, [adapter.source, apply, capture, syncToServer]);

  const diagnostics = useMemo(
    () =>
      createAppearanceDiagnostics(preferences, {
        adapterSource: adapter.source,
        reducedMotion: environment.reducedMotion,
        forcedColors: environment.forcedColors,
      }),
    [adapter.source, environment.forcedColors, environment.reducedMotion, preferences],
  );

  const value = useMemo<AppearanceSyncContextValue>(
    () => ({
      preferences,
      diagnostics,
      isReady,
      selectSkin,
      selectAppearance,
      reset,
      retry,
    }),
    [diagnostics, isReady, preferences, reset, retry, selectAppearance, selectSkin],
  );

  return (
    <AppearanceSyncContext.Provider value={value}>
      {children}
    </AppearanceSyncContext.Provider>
  );
}

export function useAppearancePreferences(): AppearanceSyncContextValue {
  const value = useContext(AppearanceSyncContext);
  if (!value) {
    throw new Error(
      "useAppearancePreferences must be used within AppearanceSyncBridge",
    );
  }
  return value;
}
