import {
  APPEARANCE_IDS,
  SKIN_IDS,
  type AppearancePreferences,
  type AppearanceSyncState,
  type RequestedAppearance,
  type SkinId,
} from "@multica/core/appearance";

export const SKIN_OPTIONS = SKIN_IDS.map((value) => ({ value }));
export const APPEARANCE_OPTIONS = APPEARANCE_IDS.map((value) => ({ value }));

export type AppearanceSyncStatus = AppearanceSyncState["status"];
export type AppearanceSyncMessage =
  | "default"
  | "local_only"
  | "pending"
  | "synced"
  | "failed";
const SYNC_MESSAGES: Readonly<
  Record<AppearanceSyncStatus, Exclude<AppearanceSyncMessage, "default">>
> = {
  "local-only": "local_only",
  pending: "pending",
  synced: "synced",
  failed: "failed",
};

export function isSkinId(value: unknown): value is SkinId {
  return SKIN_IDS.some((skin) => skin === value);
}

export function isRequestedAppearance(
  value: unknown,
): value is RequestedAppearance {
  return APPEARANCE_IDS.some((appearance) => appearance === value);
}

export function getAppearanceSyncMessage(
  preferences: Pick<AppearancePreferences, "source" | "syncState">,
): AppearanceSyncMessage {
  const { status } = preferences.syncState;
  return preferences.source === "default" && status === "local-only"
    ? "default"
    : SYNC_MESSAGES[status];
}
