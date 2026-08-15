import { useColorScheme as useNativewindColorScheme } from "nativewind";
import { useCallback, useEffect } from "react";
import * as SecureStore from "expo-secure-store";
import { create } from "zustand";
import {
  NAV_THEMES,
  SKIN_IDS,
  THEMES,
  type AppColorScheme,
  type AppSkin,
} from "@/lib/theme";
import {
  readStoredAppearance,
  SKIN_STORAGE_KEY,
  THEME_STORAGE_KEY,
  type ThemePreference,
} from "@/lib/appearance-preferences";

export type { ThemePreference } from "@/lib/appearance-preferences";

type AppearanceState = {
  preference: ThemePreference;
  skin: AppSkin;
  setPreference: (preference: ThemePreference) => void;
  setSkin: (skin: AppSkin) => void;
};

const useAppearanceState = create<AppearanceState>((set) => ({
  preference: "system",
  skin: "tension",
  setPreference: (preference) => set({ preference }),
  setSkin: (skin) => set({ skin }),
}));

let hydrationStarted = false;

export function useColorScheme() {
  const { colorScheme, setColorScheme: applyScheme } =
    useNativewindColorScheme();
  const preference = useAppearanceState((state) => state.preference);
  const skin = useAppearanceState((state) => state.skin);
  const setPreferenceState = useAppearanceState((state) => state.setPreference);
  const setSkinState = useAppearanceState((state) => state.setSkin);

  useEffect(() => {
    if (hydrationStarted) return;
    hydrationStarted = true;
    void readStoredAppearance(SecureStore.getItemAsync)
      .then(({ preference: savedTheme, skin: savedSkin, shouldRetry }) => {
        if (savedTheme) {
          setPreferenceState(savedTheme);
          applyScheme(savedTheme);
        }
        if (savedSkin) setSkinState(savedSkin);
        if (shouldRetry) hydrationStarted = false;
      })
      .catch(() => {
        hydrationStarted = false;
      });
  }, [applyScheme, setPreferenceState, setSkinState]);

  const setPreference = useCallback(
    (next: ThemePreference) => {
      setPreferenceState(next);
      applyScheme(next);
      void SecureStore.setItemAsync(THEME_STORAGE_KEY, next);
    },
    [applyScheme, setPreferenceState],
  );

  const setSkin = useCallback(
    (next: AppSkin) => {
      setSkinState(next);
      void SecureStore.setItemAsync(SKIN_STORAGE_KEY, next);
    },
    [setSkinState],
  );

  const resolvedScheme: AppColorScheme = colorScheme === "dark" ? "dark" : "light";

  return {
    colorScheme: resolvedScheme,
    preference,
    setPreference,
    skin,
    setSkin,
    skins: SKIN_IDS,
    theme: THEMES[skin][resolvedScheme],
    navigationTheme: NAV_THEMES[skin][resolvedScheme],
    isDarkColorScheme: resolvedScheme === "dark",
  };
}
