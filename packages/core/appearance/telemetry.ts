import type { AppearanceAdapterSource } from "./adapter";
import type {
  AppearancePreferenceField,
  AppearanceSyncErrorClass,
  RequestedAppearance,
  ResolvedAppearance,
  SkinId,
} from "./preferences";

export interface AppearanceAnalyticsPropertyMap {
  readonly appearance_viewed: Readonly<{
    skin: SkinId;
    requestedAppearance: RequestedAppearance;
    resolvedAppearance: ResolvedAppearance;
    adapterSource: AppearanceAdapterSource;
  }>;
  readonly skin_selected: Readonly<{
    skin: SkinId;
    previousSkin: SkinId;
    adapterSource: AppearanceAdapterSource;
  }>;
  readonly appearance_selected: Readonly<{
    appearance: RequestedAppearance;
    previousAppearance: RequestedAppearance;
    adapterSource: AppearanceAdapterSource;
  }>;
  readonly reset: Readonly<{
    adapterSource: AppearanceAdapterSource;
  }>;
  readonly sync_failed: Readonly<{
    adapterSource: AppearanceAdapterSource;
    errorClass: AppearanceSyncErrorClass;
  }>;
  readonly sync_recovered: Readonly<{
    adapterSource: AppearanceAdapterSource;
    previousErrorClass: AppearanceSyncErrorClass;
  }>;
  readonly invalid_value_recovered: Readonly<{
    adapterSource: AppearanceAdapterSource;
    field: AppearancePreferenceField;
  }>;
}

export type AppearanceAnalyticsEventName = keyof AppearanceAnalyticsPropertyMap;

type NoExtraProperties<Shape, Candidate extends Shape> = Candidate &
  Record<Exclude<keyof Candidate, keyof Shape>, never>;

export type AppearanceAnalyticsEvent = {
  [Name in AppearanceAnalyticsEventName]: Readonly<{
    name: Name;
    properties: AppearanceAnalyticsPropertyMap[Name];
  }>;
}[AppearanceAnalyticsEventName];

export interface AppearanceAnalyticsCapture {
  <
    Name extends AppearanceAnalyticsEventName,
    Properties extends AppearanceAnalyticsPropertyMap[Name],
  >(
    name: Name,
    properties: NoExtraProperties<
      AppearanceAnalyticsPropertyMap[Name],
      Properties
    >,
  ): void;
}

export function createAppearanceAnalyticsEvent<
  Name extends AppearanceAnalyticsEventName,
  Properties extends AppearanceAnalyticsPropertyMap[Name],
>(
  name: Name,
  properties: NoExtraProperties<AppearanceAnalyticsPropertyMap[Name], Properties>,
): Readonly<{ name: Name; properties: AppearanceAnalyticsPropertyMap[Name] }> {
  return { name, properties };
}
