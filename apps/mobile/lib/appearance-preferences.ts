import type { AppSkin } from "@/lib/theme";

const THEME_STORAGE_KEY = "theme-preference";
const SKIN_STORAGE_KEY = "skin-preference";

export type ThemePreference = "light" | "dark" | "system";

type StoredAppearance = {
  preference?: ThemePreference;
  skin?: AppSkin;
  shouldRetry: boolean;
};

function parseThemePreference(value: string | null): ThemePreference {
  if (value === "light" || value === "dark" || value === "system") return value;
  return "system";
}

function parseSkin(value: string | null): AppSkin {
  if (value === "tension" || value === "relay" || value === "field") return value;
  return "tension";
}

export async function readStoredAppearance(
  readItem: (key: string) => Promise<string | null>,
): Promise<StoredAppearance> {
  const [themeResult, skinResult] = await Promise.allSettled([
    readItem(THEME_STORAGE_KEY),
    readItem(SKIN_STORAGE_KEY),
  ]);

  return {
    preference:
      themeResult.status === "fulfilled"
        ? parseThemePreference(themeResult.value)
        : undefined,
    skin:
      skinResult.status === "fulfilled" ? parseSkin(skinResult.value) : undefined,
    shouldRetry:
      themeResult.status === "rejected" || skinResult.status === "rejected",
  };
}

export { SKIN_STORAGE_KEY, THEME_STORAGE_KEY };
