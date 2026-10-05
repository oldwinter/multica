import type {
  AppearancePreferenceField,
  AppearancePreferences,
  AppearanceSyncErrorClass,
} from "./preferences";
import type { AppearanceAdapterSource } from "./adapter";

export interface AppearanceDiagnosticsContext {
  readonly adapterSource: AppearanceAdapterSource;
  readonly reducedMotion: boolean;
  readonly forcedColors: boolean;
  readonly recoveredFields?: readonly AppearancePreferenceField[];
}

export interface AppearanceDiagnosticsSnapshot {
  readonly preferenceVersion: AppearancePreferences["version"];
  readonly tokenContractVersion: AppearancePreferences["tokenContractVersion"];
  readonly skin: AppearancePreferences["skin"];
  readonly requestedAppearance: AppearancePreferences["requestedAppearance"];
  readonly resolvedAppearance: AppearancePreferences["resolvedAppearance"];
  readonly preferenceSource: AppearancePreferences["source"];
  readonly adapterSource: AppearanceAdapterSource;
  readonly syncStatus: AppearancePreferences["syncState"]["status"];
  readonly lastSyncErrorClass: AppearanceSyncErrorClass | null;
  readonly reducedMotion: boolean;
  readonly forcedColors: boolean;
  readonly recoveredFields: readonly AppearancePreferenceField[];
}

export function createAppearanceDiagnostics(
  preferences: AppearancePreferences,
  context: AppearanceDiagnosticsContext,
): AppearanceDiagnosticsSnapshot {
  return {
    preferenceVersion: preferences.version,
    tokenContractVersion: preferences.tokenContractVersion,
    skin: preferences.skin,
    requestedAppearance: preferences.requestedAppearance,
    resolvedAppearance: preferences.resolvedAppearance,
    preferenceSource: preferences.source,
    adapterSource: context.adapterSource,
    syncStatus: preferences.syncState.status,
    lastSyncErrorClass:
      preferences.syncState.status === "failed"
        ? preferences.syncState.errorClass
        : null,
    reducedMotion: context.reducedMotion,
    forcedColors: context.forcedColors,
    recoveredFields: context.recoveredFields ?? [],
  };
}

export function serializeAppearanceDiagnostics(
  snapshot: AppearanceDiagnosticsSnapshot,
): string {
  return JSON.stringify(snapshot, null, 2);
}
