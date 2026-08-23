import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  parseAppearancePreferences,
  resolveAppearance,
  type AppearancePreferenceField,
  type AppearancePreferences,
  type AppearanceSyncErrorClass,
  type RequestedAppearance,
  type ResolvedAppearance,
  type SkinId,
} from "@multica/core/appearance";

export interface AppearanceUpdateRequest {
  readonly skin: SkinId;
  readonly appearance: RequestedAppearance;
  readonly appearanceUpdatedAt: string;
  readonly appearanceTokenVersion: number;
}

export interface ServerAppearanceResult {
  readonly preferences: AppearancePreferences | null;
  readonly writable: boolean;
  readonly recoveredFields: readonly AppearancePreferenceField[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function readServerAppearance(
  user: unknown,
  systemAppearance: ResolvedAppearance,
): ServerAppearanceResult {
  if (!isRecord(user)) {
    return { preferences: null, writable: true, recoveredFields: [] };
  }

  const skin = user.skin;
  const appearance = user.appearance;
  const updatedAt = user.appearanceUpdatedAt;
  const tokenContractVersion = user.appearanceTokenVersion;
  if (
    skin == null &&
    appearance == null &&
    updatedAt == null &&
    tokenContractVersion == null
  ) {
    return { preferences: null, writable: true, recoveredFields: [] };
  }

  if (
    typeof tokenContractVersion === "number" &&
    tokenContractVersion > APPEARANCE_TOKEN_CONTRACT_VERSION
  ) {
    return {
      preferences: null,
      writable: false,
      recoveredFields: ["tokenContractVersion"],
    };
  }

  const resolvableAppearance: RequestedAppearance =
    appearance === "light" || appearance === "dark" || appearance === "system"
      ? appearance
      : "system";
  const parsed = parseAppearancePreferences(
    {
      version: APPEARANCE_PREFERENCES_VERSION,
      tokenContractVersion,
      skin,
      requestedAppearance: appearance,
      resolvedAppearance: resolveAppearance(
        resolvableAppearance,
        systemAppearance,
      ),
      source: "server",
      updatedAt,
      syncState: { status: "synced" },
    },
    { systemAppearance },
  );

  return {
    preferences: {
      ...parsed.preferences,
      source: "server",
      syncState: { status: "synced" },
    },
    writable: true,
    recoveredFields: parsed.issues.map((issue) => issue.field),
  };
}

export function toAppearanceUpdateRequest(
  preferences: AppearancePreferences,
): AppearanceUpdateRequest {
  return {
    skin: preferences.skin,
    appearance: preferences.requestedAppearance,
    appearanceUpdatedAt: preferences.updatedAt,
    appearanceTokenVersion: preferences.tokenContractVersion,
  };
}

export function classifyAppearanceSyncError(
  error: unknown,
): AppearanceSyncErrorClass {
  if (isRecord(error) && typeof error.status === "number") {
    if (error.status === 401) return "unauthorized";
    if (error.status === 409) return "conflict";
    if (error.status >= 500) return "server";
  }
  if (error instanceof Error) {
    const message = error.message.toLowerCase();
    if (
      error.name === "AbortError" ||
      message.includes("network") ||
      message.includes("fetch") ||
      message.includes("timeout")
    ) {
      return "network";
    }
  }
  return "unknown";
}
