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
  withResolvedAppearance,
  type AppearanceAnalyticsCapture,
  type AppearanceDiagnosticsSnapshot,
  type AppearanceEnvironment,
  type AppearancePreferenceAdapter,
  type AppearancePreferenceField,
  type AppearancePreferences,
  type AppearanceSyncErrorClass,
  type AppearanceUndoReceipt,
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
  incomplete: boolean;
};

type AppearanceSyncContextValue = {
  preferences: AppearancePreferences;
  diagnostics: AppearanceDiagnosticsSnapshot;
  isReady: boolean;
  canRetry: boolean;
  canCopyDiagnostics: boolean;
  recoveryNoticePending: boolean;
  selectSkin: (skin: SkinId) => AppearanceUndoReceipt | null;
  selectAppearance: (
    appearance: RequestedAppearance,
  ) => AppearanceUndoReceipt | null;
  reset: () => AppearanceUndoReceipt;
  undo: (receipt: AppearanceUndoReceipt) => Promise<"applied" | "expired">;
  retry: () => void;
  acknowledgeRecoveryNotice: () => void;
};

type AppearancePersistScope = "bootstrap" | "account";
type AppearanceAccountUpdate = Required<
  Pick<
    UpdateMeRequest,
    "skin" | "appearance" | "appearanceUpdatedAt" | "appearanceTokenVersion"
  >
> &
  Pick<UpdateMeRequest, "appearanceExpectedUpdatedAt">;

type ServerSyncResult = "applied" | "expired" | "ignored" | "failed";

type PendingUndo = {
  readonly submittedUpdatedAt: string;
  readonly expectedUpdatedAt: string;
};

const DIAGNOSTICS_FAILURE_THRESHOLD = 2;

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
    return { preferences: null, writable: true, incomplete: false };
  }
  if (values.some((value) => value === null)) {
    return { preferences: null, writable: true, incomplete: true };
  }
  if (user.appearanceTokenVersion !== APPEARANCE_TOKEN_CONTRACT_VERSION) {
    return { preferences: null, writable: false, incomplete: false };
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
    ? { preferences: null, writable: false, incomplete: false }
    : { preferences: parsed.preferences, writable: true, incomplete: false };
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

function appearanceSyncRequestKey(
  accountId: string,
  preferences: AppearancePreferences,
  expectedUpdatedAt?: string,
): string {
  return [
    appearanceSyncKey(accountId, preferences),
    expectedUpdatedAt ?? "unconditional",
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
  const [syncFailureCount, setSyncFailureCount] = useState(0);
  const [recoveredFields, setRecoveredFields] = useState<
    readonly AppearancePreferenceField[]
  >([]);
  const [recoveryNoticePending, setRecoveryNoticePending] = useState(false);
  const preferencesRef = useRef(preferences);
  const environmentRef = useRef(environment);
  const remoteWritableRef = useRef(true);
  const localCacheWritableRef = useRef(true);
  const syncingRequests = useRef(new Set<string>());
  const appearanceOperationSequence = useRef(0);
  const pendingUndoRef = useRef<PendingUndo | null>(null);
  const recoveryNoticeAcknowledgedRef = useRef(false);
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

  const recordSyncFailure = useCallback(
    (errorClass: AppearanceSyncErrorClass) => {
      setSyncFailureCount((count) =>
        Math.min(DIAGNOSTICS_FAILURE_THRESHOLD, count + 1),
      );
      capture("sync_failed", {
        adapterSource: adapter.source,
        errorClass,
      });
    },
    [adapter.source, capture],
  );

  const recordRecoveredFields = useCallback(
    (fields: readonly AppearancePreferenceField[]) => {
      if (fields.length === 0) return;
      setRecoveredFields((current) =>
        Array.from(new Set([...current, ...fields])),
      );
      if (!recoveryNoticeAcknowledgedRef.current) {
        setRecoveryNoticePending(true);
      }
    },
    [],
  );

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
        recordSyncFailure("storage");
        return false;
      }
    },
    [adapter, publish, recordSyncFailure],
  );

  const syncToServer = useCallback(
    async (
      submitted: AppearancePreferences,
      options: { expectedUpdatedAt?: string } = {},
    ): Promise<ServerSyncResult> => {
      const currentUser = userRef.current;
      if (
        !currentUser ||
        !adapter.supportsRemoteSync ||
        !updateAccountAppearance ||
        !remoteWritableRef.current ||
        !localCacheWritableRef.current
      ) {
        return "ignored";
      }
      if (!environmentRef.current.online) return "ignored";

      const accountId = currentUser.id;
      const requestKey = appearanceSyncRequestKey(
        accountId,
        submitted,
        options.expectedUpdatedAt,
      );
      if (syncingRequests.current.has(requestKey)) return "ignored";

      syncingRequests.current.add(requestKey);
      try {
        const updatedUser = await updateAccountAppearance({
          skin: submitted.skin,
          appearance: submitted.requestedAppearance,
          appearanceUpdatedAt: submitted.updatedAt,
          appearanceTokenVersion: submitted.tokenContractVersion,
          ...(options.expectedUpdatedAt
            ? { appearanceExpectedUpdatedAt: options.expectedUpdatedAt }
            : {}),
        });
        if (
          userRef.current?.id !== accountId ||
          updatedUser.id !== accountId
        ) {
          return "ignored";
        }
        appearanceOperationSequence.current += 1;

        // A newer local click owns the screen; its own request will settle it.
        if (!sameExplicitPreference(preferencesRef.current, submitted)) {
          return options.expectedUpdatedAt ? "expired" : "ignored";
        }
        const remote = serverAppearanceFromUser(
          updatedUser,
          environmentRef.current.systemAppearance,
        );
        if (!remote.preferences) {
          remoteWritableRef.current = remote.writable;
          if (remote.incomplete) {
            const failed = markAppearanceSyncFailed(submitted, "server");
            await apply(failed, {
              animate: false,
              persist: true,
              accountId,
            });
            recordSyncFailure("server");
          }
          return "failed";
        }

        if (options.expectedUpdatedAt) {
          pendingUndoRef.current = null;
          if (!sameExplicitPreference(remote.preferences, submitted)) {
            await apply(markAppearanceSynced(remote.preferences), {
              animate: true,
              persist: true,
              accountId,
            });
            setSyncFailureCount(0);
            return "expired";
          }
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
        if (persisted) setSyncFailureCount(0);
        return "applied";
      } catch (error) {
        if (
          userRef.current?.id !== accountId ||
          !sameExplicitPreference(preferencesRef.current, submitted)
        ) {
          return options.expectedUpdatedAt ? "expired" : "ignored";
        }
        const errorClass = classifySyncError(error);
        const failed = markAppearanceSyncFailed(submitted, errorClass);
        await apply(failed, {
          animate: false,
          persist: true,
          accountId,
        });
        recordSyncFailure(errorClass);
        return "failed";
      } finally {
        syncingRequests.current.delete(requestKey);
      }
    },
    [adapter, apply, capture, recordSyncFailure, updateAccountAppearance],
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
      recordRecoveredFields(parsed.issues.map((issue) => issue.field));
      for (const issue of parsed.issues) {
        capture("invalid_value_recovered", {
          adapterSource: adapter.source,
          field: issue.field,
        });
      }
      if (storageFailed || futureContract) {
        recordSyncFailure(storageFailed ? "storage" : "conflict");
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
  }, [adapter, apply, capture, recordRecoveredFields, recordSyncFailure]);

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
    const pendingUndo = pendingUndoRef.current;
    const current = preferencesRef.current;
    // The auth store publishes a PATCH result before its promise resolves.
    // Keep the matching conditional Undo as the only response reconciler.
    if (
      pendingUndo?.submittedUpdatedAt === current.updatedAt &&
      syncingRequests.current.has(
        appearanceSyncRequestKey(
          accountId,
          current,
          pendingUndo.expectedUpdatedAt,
        ),
      )
    ) {
      return;
    }
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
      if (remote.incomplete) {
        next = markAppearanceSyncFailed(next, "server");
      } else if (!remote.writable) {
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

      recordRecoveredFields(parsed.issues.map((issue) => issue.field));
      for (const issue of parsed.issues) {
        capture("invalid_value_recovered", {
          adapterSource: adapter.source,
          field: issue.field,
        });
      }
      if (storageFailed || futureContract) {
        recordSyncFailure(storageFailed ? "storage" : "conflict");
      } else if (remote.incomplete) {
        recordSyncFailure("server");
      }
      if (
        remote.writable &&
        !remote.incomplete &&
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
  }, [
    adapter,
    apply,
    capture,
    isReady,
    recordRecoveredFields,
    recordSyncFailure,
    syncToServer,
    user,
    userAppearanceKey,
  ]);

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
            )
          : { preferences: null, writable: true, incomplete: false };
        const reconciled = reconcileAppearancePreferences(
          futureContract ? null : parsed.preferences,
          remote.preferences,
          environmentRef.current.systemAppearance,
        );
        const next = futureContract
          ? markAppearanceSyncFailed(reconciled.preferences, "conflict")
          : remote.incomplete
            ? markAppearanceSyncFailed(reconciled.preferences, "server")
            : reconciled.preferences;
        void apply(next, {
          animate: false,
          persist:
            !futureContract &&
            (reconciled.shouldPersistLocal || parsed.recovered),
          accountId: event.accountId ?? userRef.current?.id,
        }).then(() => {
          if (
            !futureContract &&
            !remote.incomplete &&
            reconciled.shouldSyncServer
          ) {
            void syncToServer(reconciled.preferences);
          }
        });
        recordRecoveredFields(parsed.issues.map((issue) => issue.field));
        for (const issue of parsed.issues) {
          capture("invalid_value_recovered", {
            adapterSource: adapter.source,
            field: issue.field,
          });
        }
        if (futureContract) {
          recordSyncFailure("conflict");
        }
        return;
      }
      if (event.type === "storage-error") {
        const failed = markAppearanceSyncFailed(
          preferencesRef.current,
          "storage",
        );
        publish(failed);
        recordSyncFailure("storage");
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
              const pendingUndo = pendingUndoRef.current;
              void syncToServer(current, {
                expectedUpdatedAt:
                  pendingUndo?.submittedUpdatedAt === current.updatedAt
                    ? pendingUndo.expectedUpdatedAt
                    : undefined,
              });
            }
          });
      }
    });
  }, [
    adapter,
    apply,
    capture,
    publish,
    recordRecoveredFields,
    recordSyncFailure,
    refreshAccountAppearance,
    syncToServer,
  ]);

  const commitChange = useCallback(
    (
      change: { skin?: SkinId; requestedAppearance?: RequestedAppearance },
    ): AppearanceUndoReceipt | null => {
      const current = preferencesRef.current;
      const next = changeAppearancePreferences(current, change, {
        updatedAt: nextAppearancePreferenceTimestamp(current.updatedAt),
        systemAppearance: environmentRef.current.systemAppearance,
      });
      if (next === current) return null;
      appearanceOperationSequence.current += 1;
      pendingUndoRef.current = null;
      setSyncFailureCount(0);
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
      return createAppearanceUndoReceipt(current, local);
    },
    [adapter.supportsRemoteSync, apply, syncToServer],
  );

  const selectSkin = useCallback(
    (skin: SkinId) => {
      const previousSkin = preferencesRef.current.skin;
      if (skin === previousSkin) return null;
      const receipt = commitChange({ skin });
      capture("skin_selected", {
        skin,
        previousSkin,
        adapterSource: adapter.source,
      });
      return receipt;
    },
    [adapter.source, capture, commitChange],
  );

  const selectAppearance = useCallback(
    (appearance: RequestedAppearance) => {
      const previousAppearance = preferencesRef.current.requestedAppearance;
      if (appearance === previousAppearance) return null;
      const receipt = commitChange({ requestedAppearance: appearance });
      capture("appearance_selected", {
        appearance,
        previousAppearance,
        adapterSource: adapter.source,
      });
      return receipt;
    },
    [adapter.source, capture, commitChange],
  );

  const reset = useCallback((): AppearanceUndoReceipt => {
    const current = preferencesRef.current;
    const resetPreferences = resetAppearancePreferences(
      nextAppearancePreferenceTimestamp(current.updatedAt),
      environmentRef.current.systemAppearance,
    );
    appearanceOperationSequence.current += 1;
    pendingUndoRef.current = null;
    setSyncFailureCount(0);
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
    return createAppearanceUndoReceipt(current, next);
  }, [
    adapter.source,
    adapter.supportsRemoteSync,
    apply,
    capture,
    syncToServer,
  ]);

  const undo = useCallback(
    async (
      receipt: AppearanceUndoReceipt,
    ): Promise<"applied" | "expired"> => {
      const current = preferencesRef.current;
      const result = undoAppearancePreferences(current, receipt, {
        updatedAt: nextAppearancePreferenceTimestamp(current.updatedAt),
        systemAppearance: environmentRef.current.systemAppearance,
      });
      if (result.status === "expired") return "expired";

      appearanceOperationSequence.current += 1;
      setSyncFailureCount(0);
      const canSyncRemotely =
        Boolean(userRef.current) &&
        adapter.supportsRemoteSync &&
        Boolean(updateAccountAppearance) &&
        remoteWritableRef.current &&
        localCacheWritableRef.current;
      const next = canSyncRemotely
        ? result.preferences
        : {
            ...result.preferences,
            syncState: { status: "local-only" as const },
          };
      pendingUndoRef.current = canSyncRemotely
        ? {
            submittedUpdatedAt: next.updatedAt,
            expectedUpdatedAt: receipt.expectedUpdatedAt,
          }
        : null;
      const accountId = userRef.current?.id;
      await apply(next, {
        animate: true,
        persist: true,
        accountId,
      });
      const syncResult = await syncToServer(next, {
        expectedUpdatedAt: pendingUndoRef.current?.expectedUpdatedAt,
      });
      return syncResult === "expired" ? "expired" : "applied";
    },
    [
      adapter.supportsRemoteSync,
      apply,
      syncToServer,
      updateAccountAppearance,
    ],
  );

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
            setSyncFailureCount(0);
            if (retrying.syncState.status === "pending") {
              const pendingUndo = pendingUndoRef.current;
              void syncToServer(retrying, {
                expectedUpdatedAt:
                  pendingUndo?.submittedUpdatedAt === retrying.updatedAt
                    ? pendingUndo.expectedUpdatedAt
                    : undefined,
              });
            }
          }
        },
      );
      return;
    }
    const pendingUndo = pendingUndoRef.current;
    void syncToServer(current, {
      expectedUpdatedAt:
        pendingUndo?.submittedUpdatedAt === current.updatedAt
          ? pendingUndo.expectedUpdatedAt
          : undefined,
    });
  }, [adapter.source, apply, capture, syncToServer]);

  const acknowledgeRecoveryNotice = useCallback(() => {
    recoveryNoticeAcknowledgedRef.current = true;
    setRecoveryNoticePending(false);
  }, []);

  const diagnostics = useMemo(
    () =>
      createAppearanceDiagnostics(preferences, {
        adapterSource: adapter.source,
        reducedMotion: environment.reducedMotion,
        forcedColors: environment.forcedColors,
        recoveredFields,
      }),
    [
      adapter.source,
      environment.forcedColors,
      environment.reducedMotion,
      preferences,
      recoveredFields,
    ],
  );

  const canRetry =
    preferences.syncState.status === "failed" &&
    (preferences.syncState.errorClass === "storage" ||
      (environment.online &&
        Boolean(user) &&
        adapter.supportsRemoteSync &&
        Boolean(updateAccountAppearance) &&
        remoteWritableRef.current &&
        localCacheWritableRef.current));

  const value = useMemo<AppearanceSyncContextValue>(
    () => ({
      preferences,
      diagnostics,
      isReady,
      canRetry,
      canCopyDiagnostics:
        syncFailureCount >= DIAGNOSTICS_FAILURE_THRESHOLD,
      recoveryNoticePending,
      selectSkin,
      selectAppearance,
      reset,
      undo,
      retry,
      acknowledgeRecoveryNotice,
    }),
    [
      acknowledgeRecoveryNotice,
      canRetry,
      diagnostics,
      isReady,
      preferences,
      recoveryNoticePending,
      reset,
      retry,
      selectAppearance,
      selectSkin,
      syncFailureCount,
      undo,
    ],
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
