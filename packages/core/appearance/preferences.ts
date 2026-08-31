export const SKIN_IDS = ["tension", "relay", "field"] as const;
export type SkinId = (typeof SKIN_IDS)[number];

export const APPEARANCE_IDS = ["system", "light", "dark"] as const;
export type RequestedAppearance = (typeof APPEARANCE_IDS)[number];
export type ResolvedAppearance = Exclude<RequestedAppearance, "system">;

export const DEFAULT_SKIN: SkinId = "tension";
export const DEFAULT_APPEARANCE: RequestedAppearance = "system";
export const APPEARANCE_PREFERENCES_VERSION = 1 as const;
export const APPEARANCE_TOKEN_CONTRACT_VERSION = 1 as const;
export const APPEARANCE_EPOCH = "1970-01-01T00:00:00.000Z";

export const APPEARANCE_PREFERENCE_SOURCES = [
  "default",
  "local",
  "server",
] as const;
export type AppearancePreferenceSource =
  (typeof APPEARANCE_PREFERENCE_SOURCES)[number];

export const APPEARANCE_SYNC_ERROR_CLASSES = [
  "network",
  "unauthorized",
  "conflict",
  "server",
  "storage",
  "unknown",
] as const;
export type AppearanceSyncErrorClass =
  (typeof APPEARANCE_SYNC_ERROR_CLASSES)[number];

export type AppearanceSyncState =
  | Readonly<{ status: "local-only" }>
  | Readonly<{ status: "pending" }>
  | Readonly<{ status: "synced" }>
  | Readonly<{ status: "failed"; errorClass: AppearanceSyncErrorClass }>;

export interface AppearancePreferences {
  readonly version: typeof APPEARANCE_PREFERENCES_VERSION;
  readonly tokenContractVersion: typeof APPEARANCE_TOKEN_CONTRACT_VERSION;
  readonly skin: SkinId;
  readonly requestedAppearance: RequestedAppearance;
  readonly resolvedAppearance: ResolvedAppearance;
  readonly source: AppearancePreferenceSource;
  readonly updatedAt: string;
  readonly syncState: AppearanceSyncState;
}

export type AppearancePreferenceField =
  | "root"
  | "version"
  | "tokenContractVersion"
  | "skin"
  | "requestedAppearance"
  | "resolvedAppearance"
  | "source"
  | "updatedAt"
  | "syncState";

export type AppearancePreferenceIssueCode =
  | "invalid_type"
  | "invalid_value"
  | "inconsistent_value"
  | "unknown_field"
  | "unsupported_version";

export interface AppearancePreferenceIssue {
  readonly field: AppearancePreferenceField;
  readonly code: AppearancePreferenceIssueCode;
}

export interface ParseAppearancePreferencesOptions {
  readonly systemAppearance: ResolvedAppearance;
}

export interface AppearancePreferenceParseResult {
  readonly preferences: AppearancePreferences;
  readonly recovered: boolean;
  readonly issues: readonly AppearancePreferenceIssue[];
}

export type AppearancePreferenceValidationResult =
  | Readonly<{ valid: true; value: AppearancePreferences }>
  | Readonly<{ valid: false; issues: readonly AppearancePreferenceIssue[] }>;

export interface AppearancePreferenceChange {
  readonly skin?: SkinId;
  readonly requestedAppearance?: RequestedAppearance;
}

export interface AppearancePreferenceChangeOptions {
  readonly updatedAt: string;
  readonly systemAppearance: ResolvedAppearance;
}

export interface AppearanceUndoReceipt {
  readonly previous: AppearancePreferences;
  readonly expectedUpdatedAt: string;
}

export type AppearanceUndoResult =
  | Readonly<{
      status: "applied";
      preferences: AppearancePreferences;
      expectedUpdatedAt: string;
    }>
  | Readonly<{
      status: "expired";
      preferences: AppearancePreferences;
    }>;

export type AppearanceReconciliationWinner = "default" | "local" | "server";

export interface AppearanceReconciliationResult {
  readonly preferences: AppearancePreferences;
  readonly winner: AppearanceReconciliationWinner;
  readonly shouldPersistLocal: boolean;
  readonly shouldSyncServer: boolean;
}

const PREFERENCE_KEYS = new Set([
  "version",
  "tokenContractVersion",
  "skin",
  "requestedAppearance",
  "resolvedAppearance",
  "source",
  "updatedAt",
  "syncState",
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function includes<T extends string>(values: readonly T[], value: unknown): value is T {
  return typeof value === "string" && values.includes(value as T);
}

interface ParsedTimestamp {
  readonly canonical: string;
  readonly epochNanoseconds: bigint;
}

const RFC3339_TIMESTAMP =
  /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(Z|([+-])(\d{2}):(\d{2}))$/;

function daysInMonth(year: number, month: number): number {
  if (month === 2) {
    const leap = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
    return leap ? 29 : 28;
  }
  return [4, 6, 9, 11].includes(month) ? 30 : 31;
}

function parseTimestamp(value: unknown): ParsedTimestamp | null {
  if (typeof value !== "string") return null;
  const match = RFC3339_TIMESTAMP.exec(value);
  if (!match) return null;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const hour = Number(match[4]);
  const minute = Number(match[5]);
  const second = Number(match[6]);
  const fraction = match[7] ?? "";
  const offsetSign = match[9];
  const offsetHour = Number(match[10] ?? 0);
  const offsetMinute = Number(match[11] ?? 0);

  if (
    year < 1 ||
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > daysInMonth(year, month) ||
    hour > 23 ||
    minute > 59 ||
    second > 59 ||
    offsetHour > 23 ||
    offsetMinute > 59
  ) {
    return null;
  }

  const localDate = new Date(0);
  localDate.setUTCFullYear(year, month - 1, day);
  localDate.setUTCHours(hour, minute, second, 0);
  const localSeconds = BigInt(Math.trunc(localDate.getTime() / 1_000));
  const offsetMagnitude = offsetHour * 3_600 + offsetMinute * 60;
  const offsetSeconds =
    offsetSign === "+"
      ? offsetMagnitude
      : offsetSign === "-"
        ? -offsetMagnitude
        : 0;
  const epochSeconds = localSeconds - BigInt(offsetSeconds);
  const nanoseconds = fraction.padEnd(9, "0");
  const epochNanoseconds =
    epochSeconds * 1_000_000_000n + BigInt(nanoseconds || "0");

  const utcPrefix = new Date(Number(epochSeconds) * 1_000)
    .toISOString()
    .slice(0, 19);
  const significantFraction = nanoseconds.replace(/0+$/, "").padEnd(3, "0");
  return {
    canonical: `${utcPrefix}.${significantFraction}Z`,
    epochNanoseconds,
  };
}

function isSyncState(value: unknown): value is AppearanceSyncState {
  if (!isRecord(value)) return false;
  if (Object.keys(value).some((key) => key !== "status" && key !== "errorClass")) {
    return false;
  }
  if (
    value.status === "local-only" ||
    value.status === "pending" ||
    value.status === "synced"
  ) {
    return Object.keys(value).length === 1;
  }
  return (
    value.status === "failed" &&
    includes(APPEARANCE_SYNC_ERROR_CLASSES, value.errorClass) &&
    Object.keys(value).length === 2
  );
}

function sameChoice(
  first: AppearancePreferences,
  second: AppearancePreferences,
): boolean {
  return (
    first.skin === second.skin &&
    first.requestedAppearance === second.requestedAppearance
  );
}

export function resolveAppearance(
  requestedAppearance: RequestedAppearance,
  systemAppearance: ResolvedAppearance,
): ResolvedAppearance {
  return requestedAppearance === "system" ? systemAppearance : requestedAppearance;
}

export function createDefaultAppearancePreferences(
  systemAppearance: ResolvedAppearance,
): AppearancePreferences {
  return {
    version: APPEARANCE_PREFERENCES_VERSION,
    tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
    skin: DEFAULT_SKIN,
    requestedAppearance: DEFAULT_APPEARANCE,
    resolvedAppearance: systemAppearance,
    source: "default",
    updatedAt: APPEARANCE_EPOCH,
    syncState: { status: "local-only" },
  };
}

export function withResolvedAppearance(
  preferences: AppearancePreferences,
  systemAppearance: ResolvedAppearance,
): AppearancePreferences {
  const resolvedAppearance = resolveAppearance(
    preferences.requestedAppearance,
    systemAppearance,
  );
  if (preferences.resolvedAppearance === resolvedAppearance) return preferences;
  return { ...preferences, resolvedAppearance };
}

export function hasFutureAppearanceContractVersion(value: unknown): boolean {
  if (!isRecord(value)) return false;
  return (
    (typeof value.version === "number" &&
      Number.isInteger(value.version) &&
      value.version > APPEARANCE_PREFERENCES_VERSION) ||
    (typeof value.tokenContractVersion === "number" &&
      Number.isInteger(value.tokenContractVersion) &&
      value.tokenContractVersion > APPEARANCE_TOKEN_CONTRACT_VERSION)
  );
}

/** Keep consecutive explicit choices strictly ordered even within one tick. */
export function nextAppearancePreferenceTimestamp(
  current: string,
  nowMilliseconds = Date.now(),
): string {
  const parsedCurrent = parseTimestamp(current);
  if (!parsedCurrent || !Number.isFinite(nowMilliseconds)) {
    throw new TypeError("appearance timestamp inputs must be valid");
  }
  const currentMilliseconds = Number(
    parsedCurrent.epochNanoseconds / 1_000_000n,
  );
  const nextMilliseconds =
    nowMilliseconds <= currentMilliseconds
      ? currentMilliseconds + 1
      : Math.trunc(nowMilliseconds);
  return new Date(nextMilliseconds).toISOString();
}

export function validateAppearancePreferences(
  value: unknown,
): AppearancePreferenceValidationResult {
  if (!isRecord(value)) {
    return {
      valid: false,
      issues: [{ field: "root", code: "invalid_type" }],
    };
  }

  const issues: AppearancePreferenceIssue[] = [];
  if (Object.keys(value).some((key) => !PREFERENCE_KEYS.has(key))) {
    issues.push({ field: "root", code: "unknown_field" });
  }
  if (value.version !== APPEARANCE_PREFERENCES_VERSION) {
    issues.push({ field: "version", code: "unsupported_version" });
  }
  if (value.tokenContractVersion !== APPEARANCE_TOKEN_CONTRACT_VERSION) {
    issues.push({ field: "tokenContractVersion", code: "invalid_value" });
  }
  if (!includes(SKIN_IDS, value.skin)) {
    issues.push({ field: "skin", code: "invalid_value" });
  }
  if (!includes(APPEARANCE_IDS, value.requestedAppearance)) {
    issues.push({ field: "requestedAppearance", code: "invalid_value" });
  }
  if (!includes(["light", "dark"] as const, value.resolvedAppearance)) {
    issues.push({ field: "resolvedAppearance", code: "invalid_value" });
  } else if (
    (value.requestedAppearance === "light" || value.requestedAppearance === "dark") &&
    value.resolvedAppearance !== value.requestedAppearance
  ) {
    issues.push({ field: "resolvedAppearance", code: "inconsistent_value" });
  }
  if (!includes(APPEARANCE_PREFERENCE_SOURCES, value.source)) {
    issues.push({ field: "source", code: "invalid_value" });
  }
  const parsedTimestamp = parseTimestamp(value.updatedAt);
  if (!parsedTimestamp) {
    issues.push({ field: "updatedAt", code: "invalid_value" });
  }
  if (!isSyncState(value.syncState)) {
    issues.push({ field: "syncState", code: "invalid_value" });
  }

  if (issues.length > 0) return { valid: false, issues };
  return {
    valid: true,
    value: {
      version: APPEARANCE_PREFERENCES_VERSION,
      tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
      skin: value.skin as SkinId,
      requestedAppearance: value.requestedAppearance as RequestedAppearance,
      resolvedAppearance: value.resolvedAppearance as ResolvedAppearance,
      source: value.source as AppearancePreferenceSource,
      updatedAt: parsedTimestamp!.canonical,
      syncState: value.syncState as AppearanceSyncState,
    },
  };
}

export function parseAppearancePreferences(
  value: unknown,
  options: ParseAppearancePreferencesOptions,
): AppearancePreferenceParseResult {
  const fallback = createDefaultAppearancePreferences(options.systemAppearance);
  if (value === null || value === undefined) {
    return { preferences: fallback, recovered: false, issues: [] };
  }
  if (!isRecord(value)) {
    return {
      preferences: fallback,
      recovered: true,
      issues: [{ field: "root", code: "invalid_type" }],
    };
  }
  if (value.version !== APPEARANCE_PREFERENCES_VERSION) {
    return {
      preferences: fallback,
      recovered: true,
      issues: [{ field: "version", code: "unsupported_version" }],
    };
  }

  const issues: AppearancePreferenceIssue[] = [];
  const tokenContractVersion =
    value.tokenContractVersion === APPEARANCE_TOKEN_CONTRACT_VERSION
      ? APPEARANCE_TOKEN_CONTRACT_VERSION
      : fallback.tokenContractVersion;
  if (value.tokenContractVersion !== APPEARANCE_TOKEN_CONTRACT_VERSION) {
    issues.push({ field: "tokenContractVersion", code: "invalid_value" });
  }

  const skin = includes(SKIN_IDS, value.skin) ? value.skin : fallback.skin;
  if (!includes(SKIN_IDS, value.skin)) {
    issues.push({ field: "skin", code: "invalid_value" });
  }

  const requestedAppearance = includes(APPEARANCE_IDS, value.requestedAppearance)
    ? value.requestedAppearance
    : fallback.requestedAppearance;
  if (!includes(APPEARANCE_IDS, value.requestedAppearance)) {
    issues.push({ field: "requestedAppearance", code: "invalid_value" });
  }

  const resolvedAppearance = resolveAppearance(
    requestedAppearance,
    options.systemAppearance,
  );
  if (!includes(["light", "dark"] as const, value.resolvedAppearance)) {
    issues.push({ field: "resolvedAppearance", code: "invalid_value" });
  } else if (value.resolvedAppearance !== resolvedAppearance) {
    issues.push({ field: "resolvedAppearance", code: "inconsistent_value" });
  }

  const source = includes(APPEARANCE_PREFERENCE_SOURCES, value.source)
    ? value.source
    : fallback.source;
  if (!includes(APPEARANCE_PREFERENCE_SOURCES, value.source)) {
    issues.push({ field: "source", code: "invalid_value" });
  }

  const parsedTimestamp = parseTimestamp(value.updatedAt);
  const updatedAt = parsedTimestamp?.canonical ?? fallback.updatedAt;
  if (!parsedTimestamp) {
    issues.push({ field: "updatedAt", code: "invalid_value" });
  }

  const syncState = isSyncState(value.syncState)
    ? value.syncState
    : fallback.syncState;
  if (!isSyncState(value.syncState)) {
    issues.push({ field: "syncState", code: "invalid_value" });
  }

  return {
    preferences: {
      version: APPEARANCE_PREFERENCES_VERSION,
      tokenContractVersion,
      skin,
      requestedAppearance,
      resolvedAppearance,
      source,
      updatedAt,
      syncState,
    },
    recovered: issues.length > 0,
    issues,
  };
}

export function changeAppearancePreferences(
  current: AppearancePreferences,
  change: AppearancePreferenceChange,
  options: AppearancePreferenceChangeOptions,
): AppearancePreferences {
  const parsedTimestamp = parseTimestamp(options.updatedAt);
  if (!parsedTimestamp) {
    throw new TypeError("updatedAt must be a valid RFC3339 timestamp");
  }
  const skin = change.skin ?? current.skin;
  const requestedAppearance =
    change.requestedAppearance ?? current.requestedAppearance;
  if (
    skin === current.skin &&
    requestedAppearance === current.requestedAppearance
  ) {
    return withResolvedAppearance(current, options.systemAppearance);
  }
  return {
    ...current,
    skin,
    requestedAppearance,
    resolvedAppearance: resolveAppearance(
      requestedAppearance,
      options.systemAppearance,
    ),
    source: "local",
    updatedAt: parsedTimestamp.canonical,
    syncState: { status: "pending" },
  };
}

export function createAppearanceUndoReceipt(
  previous: AppearancePreferences,
  applied: AppearancePreferences,
): AppearanceUndoReceipt {
  return {
    previous,
    expectedUpdatedAt: applied.updatedAt,
  };
}

export function undoAppearancePreferences(
  current: AppearancePreferences,
  receipt: AppearanceUndoReceipt,
  options: AppearancePreferenceChangeOptions,
): AppearanceUndoResult {
  if (current.updatedAt !== receipt.expectedUpdatedAt) {
    return { status: "expired", preferences: current };
  }

  return {
    status: "applied",
    preferences: changeAppearancePreferences(
      current,
      {
        skin: receipt.previous.skin,
        requestedAppearance: receipt.previous.requestedAppearance,
      },
      options,
    ),
    expectedUpdatedAt: receipt.expectedUpdatedAt,
  };
}

export function resetAppearancePreferences(
  updatedAt: string,
  systemAppearance: ResolvedAppearance,
): AppearancePreferences {
  const parsedTimestamp = parseTimestamp(updatedAt);
  if (!parsedTimestamp) {
    throw new TypeError("updatedAt must be a valid RFC3339 timestamp");
  }
  return {
    ...createDefaultAppearancePreferences(systemAppearance),
    source: "local",
    updatedAt: parsedTimestamp.canonical,
    syncState: { status: "pending" },
  };
}

export function markAppearanceSyncFailed(
  preferences: AppearancePreferences,
  errorClass: AppearanceSyncErrorClass,
): AppearancePreferences {
  return {
    ...preferences,
    syncState: { status: "failed", errorClass },
  };
}

export function markAppearanceSynced(
  preferences: AppearancePreferences,
): AppearancePreferences {
  if (preferences.syncState.status === "synced") return preferences;
  return { ...preferences, syncState: { status: "synced" } };
}

export function reconcileAppearancePreferences(
  local: AppearancePreferences | null,
  server: AppearancePreferences | null,
  systemAppearance: ResolvedAppearance,
): AppearanceReconciliationResult {
  if (!local && !server) {
    return {
      preferences: createDefaultAppearancePreferences(systemAppearance),
      winner: "default",
      shouldPersistLocal: false,
      shouldSyncServer: false,
    };
  }

  if (!server) {
    const resolved = withResolvedAppearance(local!, systemAppearance);
    const shouldSyncServer = resolved.source === "local";
    const shouldSettleDefault =
      resolved.source === "default" &&
      resolved.syncState.status !== "local-only";
    return {
      preferences: shouldSyncServer
        ? { ...resolved, syncState: { status: "pending" } }
        : shouldSettleDefault
          ? { ...resolved, syncState: { status: "local-only" } }
          : resolved,
      winner: "local",
      shouldPersistLocal: shouldSettleDefault,
      shouldSyncServer,
    };
  }

  if (!local) {
    return {
      preferences: {
        ...withResolvedAppearance(server, systemAppearance),
        source: "server",
        syncState: { status: "synced" },
      },
      winner: "server",
      shouldPersistLocal: true,
      shouldSyncServer: false,
    };
  }

  const localTime = parseTimestamp(local.updatedAt)?.epochNanoseconds;
  const serverTime = parseTimestamp(server.updatedAt)?.epochNanoseconds;
  if (localTime === undefined || serverTime === undefined) {
    throw new TypeError("AppearancePreferences.updatedAt must be valid RFC3339");
  }
  if (serverTime > localTime || (serverTime === localTime && sameChoice(local, server))) {
    return {
      preferences: {
        ...withResolvedAppearance(server, systemAppearance),
        source: "server",
        syncState: { status: "synced" },
      },
      winner: "server",
      shouldPersistLocal:
        localTime !== serverTime ||
        local.source !== "server" ||
        local.syncState.status !== "synced",
      shouldSyncServer: false,
    };
  }

  return {
    preferences: {
      ...withResolvedAppearance(local, systemAppearance),
      source: "local",
      syncState: { status: "pending" },
    },
    winner: "local",
    shouldPersistLocal: false,
    shouldSyncServer: true,
  };
}
