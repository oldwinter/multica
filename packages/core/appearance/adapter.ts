import type {
  AppearancePreferences,
  ResolvedAppearance,
} from "./preferences";

export const APPEARANCE_ADAPTER_SOURCES = [
  "web",
  "desktop",
  "mobile",
  "docs",
] as const;
export type AppearanceAdapterSource =
  (typeof APPEARANCE_ADAPTER_SOURCES)[number];

export interface AppearanceEnvironment {
  readonly systemAppearance: ResolvedAppearance;
  readonly reducedMotion: boolean;
  readonly forcedColors: boolean;
  readonly online: boolean;
}

export type AppearanceAdapterEvent =
  | Readonly<{
      type: "system-appearance-changed";
      systemAppearance: ResolvedAppearance;
    }>
  | Readonly<{
      type: "external-preferences-changed";
      value: unknown;
      accountId?: string;
    }>
  | Readonly<{
      type: "connectivity-changed";
      online: boolean;
    }>
  | Readonly<{
      type: "storage-error";
    }>;

export type AppearanceAdapterListener = (event: AppearanceAdapterEvent) => void;
export type Awaitable<T> = T | Promise<T>;

/**
 * The platform boundary for appearance bootstrap and local durability.
 *
 * Values returned by load are intentionally unknown: platform storage is an
 * untrusted boundary and callers must pass the result through
 * parseAppearancePreferences before applying it.
 */
export interface AppearancePreferenceAdapter {
  readonly source: AppearanceAdapterSource;
  readonly supportsRemoteSync: boolean;
  load(): Awaitable<unknown | null>;
  loadForAccount?(accountId: string): Awaitable<unknown | null>;
  persist(preferences: AppearancePreferences): Awaitable<void>;
  persistForAccount?(
    accountId: string,
    preferences: AppearancePreferences,
  ): Awaitable<void>;
  apply(preferences: AppearancePreferences): Awaitable<void>;
  getEnvironment(): Awaitable<AppearanceEnvironment>;
  subscribe(listener: AppearanceAdapterListener): () => void;
}
