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
  changeAppearancePreferences,
  createAppearanceDiagnostics,
  createDefaultAppearancePreferences,
  hasFutureAppearanceContractVersion,
  markAppearanceSyncFailed,
  nextAppearancePreferenceTimestamp,
  parseAppearancePreferences,
  resetAppearancePreferences,
  withResolvedAppearance,
  type AppearanceDiagnosticsSnapshot,
  type AppearanceEnvironment,
  type AppearancePreferences,
  type RequestedAppearance,
  type SkinId,
} from "@multica/core/appearance";
import { useSkin, useTheme } from "@multica/ui/components/common/theme-provider";
import { createBrowserAppearanceAdapter } from "@multica/views/appearance";

const docsAppearanceAdapter = createBrowserAppearanceAdapter("docs");

type DocsAppearanceContextValue = {
  preferences: AppearancePreferences;
  diagnostics: AppearanceDiagnosticsSnapshot;
  selectSkin: (skin: SkinId) => void;
  selectAppearance: (appearance: RequestedAppearance) => void;
  reset: () => void;
};

const DocsAppearanceContext = createContext<DocsAppearanceContextValue | null>(
  null,
);

const INITIAL_ENVIRONMENT: AppearanceEnvironment = {
  systemAppearance: "light",
  reducedMotion: false,
  forcedColors: false,
  online: true,
};

export function DocsAppearanceProvider({ children }: { children: ReactNode }) {
  const { setSkin } = useSkin();
  const { setTheme } = useTheme();
  const setSkinRef = useRef(setSkin);
  const setThemeRef = useRef(setTheme);
  setSkinRef.current = setSkin;
  setThemeRef.current = setTheme;
  const [environment, setEnvironment] =
    useState<AppearanceEnvironment>(INITIAL_ENVIRONMENT);
  const environmentRef = useRef(environment);
  const [preferences, setPreferences] = useState<AppearancePreferences>(() =>
    createDefaultAppearancePreferences("light"),
  );
  const preferencesRef = useRef(preferences);
  const localCacheWritableRef = useRef(true);

  const apply = useCallback(
    async (
      next: AppearancePreferences,
      options: { persist: boolean; animate: boolean },
    ) => {
      preferencesRef.current = next;
      setPreferences(next);
      await docsAppearanceAdapter.apply(next);
      setSkinRef.current(next.skin, { animate: options.animate });
      setThemeRef.current(next.requestedAppearance, {
        animate: options.animate,
      });
      if (!options.persist || !localCacheWritableRef.current) return true;
      try {
        await docsAppearanceAdapter.persist(next);
        return true;
      } catch {
        const failed = markAppearanceSyncFailed(next, "storage");
        preferencesRef.current = failed;
        setPreferences(failed);
        return false;
      }
    },
    [],
  );

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      let nextEnvironment = INITIAL_ENVIRONMENT;
      let raw: unknown | null = null;
      let storageFailed = false;
      try {
        nextEnvironment = await docsAppearanceAdapter.getEnvironment();
      } catch {
        // Defaults keep the documentation readable when media queries fail.
      }
      try {
        raw = await docsAppearanceAdapter.load();
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
      let localPreferences =
        parsed.preferences.source === "local"
          ? {
              ...parsed.preferences,
              syncState: { status: "local-only" as const },
            }
          : parsed.preferences;
      if (futureContract) {
        localPreferences = markAppearanceSyncFailed(
          createDefaultAppearancePreferences(nextEnvironment.systemAppearance),
          "conflict",
        );
      } else if (storageFailed) {
        localPreferences = markAppearanceSyncFailed(
          localPreferences,
          "storage",
        );
      }
      await apply(localPreferences, {
        persist: parsed.recovered && !futureContract && !storageFailed,
        animate: false,
      });
    })();
    return () => {
      cancelled = true;
    };
  }, [apply]);

  useEffect(() => {
    return docsAppearanceAdapter.subscribe((event) => {
      if (event.type === "connectivity-changed") return;
      if (event.type === "storage-error") {
        const failed = markAppearanceSyncFailed(
          preferencesRef.current,
          "storage",
        );
        preferencesRef.current = failed;
        setPreferences(failed);
        return;
      }
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
        void apply(resolved, { persist: true, animate: false });
        return;
      }

      const futureContract = hasFutureAppearanceContractVersion(event.value);
      localCacheWritableRef.current = !futureContract;
      const parsed = parseAppearancePreferences(event.value, {
        systemAppearance: environmentRef.current.systemAppearance,
      });
      const next = futureContract
        ? markAppearanceSyncFailed(
            createDefaultAppearancePreferences(
              environmentRef.current.systemAppearance,
            ),
            "conflict",
          )
        : parsed.preferences;
      void apply(next, {
        persist: parsed.recovered && !futureContract,
        animate: false,
      });
    });
  }, [apply]);

  const commit = useCallback(
    (change: {
      skin?: SkinId;
      requestedAppearance?: RequestedAppearance;
    }) => {
      const current = preferencesRef.current;
      const changed = changeAppearancePreferences(current, change, {
        updatedAt: nextAppearancePreferenceTimestamp(current.updatedAt),
        systemAppearance: environmentRef.current.systemAppearance,
      });
      if (changed === current) return;
      const next = {
        ...changed,
        syncState: { status: "local-only" as const },
      };
      void apply(next, {
        persist: true,
        animate: !environmentRef.current.reducedMotion,
      });
    },
    [apply],
  );

  const reset = useCallback(() => {
    const resetPreferences = resetAppearancePreferences(
      nextAppearancePreferenceTimestamp(preferencesRef.current.updatedAt),
      environmentRef.current.systemAppearance,
    );
    const next = {
      ...resetPreferences,
      syncState: { status: "local-only" as const },
    };
    void apply(next, {
      persist: true,
      animate: !environmentRef.current.reducedMotion,
    });
  }, [apply]);

  const diagnostics = useMemo(
    () =>
      createAppearanceDiagnostics(preferences, {
        adapterSource: docsAppearanceAdapter.source,
        reducedMotion: environment.reducedMotion,
        forcedColors: environment.forcedColors,
      }),
    [environment.forcedColors, environment.reducedMotion, preferences],
  );

  const value = useMemo<DocsAppearanceContextValue>(
    () => ({
      preferences,
      diagnostics,
      selectSkin: (skin) => commit({ skin }),
      selectAppearance: (requestedAppearance) =>
        commit({ requestedAppearance }),
      reset,
    }),
    [commit, diagnostics, preferences, reset],
  );

  return (
    <DocsAppearanceContext.Provider value={value}>
      {children}
    </DocsAppearanceContext.Provider>
  );
}

export function useDocsAppearance(): DocsAppearanceContextValue {
  const value = useContext(DocsAppearanceContext);
  if (!value) {
    throw new Error("useDocsAppearance must be used within DocsAppearanceProvider");
  }
  return value;
}
