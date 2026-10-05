import type {
  AppConfigResponse,
} from "@multica/core/api/schemas";
import type {
  AppearanceAnalyticsCapture,
  AppearanceAnalyticsEvent,
} from "@multica/core/appearance";

const DEFAULT_POSTHOG_HOST = "https://us.i.posthog.com";
const MAX_BUFFERED_EVENTS = 32;
const APPEARANCE_ANALYTICS_TIMEOUT_MS = 10_000;

type AppearanceAnalyticsConfig = Pick<
  AppConfigResponse,
  "posthog_key" | "posthog_host" | "analytics_environment"
>;

interface PendingAppearanceEvent {
  event: AppearanceAnalyticsEvent;
  distinctId: string;
}

type AnalyticsFetch = (
  input: string,
  init: RequestInit,
) => Promise<{ ok: boolean }>;

export interface MobileAppearanceAnalyticsTransport {
  configure(config: AppearanceAnalyticsConfig): void;
  identify(userId: string | null): void;
  capture: AppearanceAnalyticsCapture;
  retry(): Promise<void>;
  pendingCount(): number;
}

export function createMobileAppearanceAnalyticsTransport(
  fetchImpl: AnalyticsFetch,
  requestTimeoutMs = APPEARANCE_ANALYTICS_TIMEOUT_MS,
): MobileAppearanceAnalyticsTransport {
  let config: AppearanceAnalyticsConfig | null = null;
  let configured = false;
  let distinctId = "mobile-anonymous";
  let flushing: Promise<void> | null = null;
  const pending: PendingAppearanceEvent[] = [];

  const flush = (): Promise<void> => {
    if (!configured || !config?.posthog_key || pending.length === 0) {
      return Promise.resolve();
    }
    if (flushing) return flushing;

    flushing = (async () => {
      while (configured && config?.posthog_key && pending.length > 0) {
        const currentConfig = config;
        const batch = pending.splice(0, pending.length);
        const host = (currentConfig.posthog_host || DEFAULT_POSTHOG_HOST).replace(
          /\/$/,
          "",
        );
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), requestTimeoutMs);
        try {
          const response = await fetchImpl(`${host}/batch/`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              api_key: currentConfig.posthog_key,
              batch: batch.map(({ event, distinctId: eventDistinctId }) => ({
                event: event.name,
                properties: {
                  ...event.properties,
                  distinct_id: eventDistinctId,
                  client_type: "mobile",
                  event_schema_version: 1,
                  environment:
                    currentConfig.analytics_environment || "dev",
                },
              })),
            }),
            signal: controller.signal,
          });
          if (!response.ok) throw new Error("Appearance analytics rejected");
        } catch {
          pending.unshift(...batch);
          if (pending.length > MAX_BUFFERED_EVENTS) {
            pending.splice(0, pending.length - MAX_BUFFERED_EVENTS);
          }
          break;
        } finally {
          clearTimeout(timeout);
        }
      }
    })().finally(() => {
      flushing = null;
    });
    return flushing;
  };

  const capture: AppearanceAnalyticsCapture = (name, properties) => {
    pending.push({
      event: { name, properties } as AppearanceAnalyticsEvent,
      distinctId,
    });
    if (pending.length > MAX_BUFFERED_EVENTS) pending.shift();
    void flush();
  };

  return {
    configure(nextConfig) {
      configured = true;
      config = nextConfig.posthog_key ? nextConfig : null;
      if (!config) {
        pending.length = 0;
        return;
      }
      void flush();
    },
    identify(userId) {
      distinctId = userId || "mobile-anonymous";
    },
    capture,
    retry: flush,
    pendingCount: () => pending.length,
  };
}

const mobileAppearanceAnalytics = createMobileAppearanceAnalyticsTransport(
  (input, init) => fetch(input, init),
);

export const captureMobileAppearanceEvent =
  mobileAppearanceAnalytics.capture;
export const configureMobileAppearanceAnalytics = (
  config: AppearanceAnalyticsConfig,
) => mobileAppearanceAnalytics.configure(config);
export const identifyMobileAppearanceAnalytics = (userId: string | null) =>
  mobileAppearanceAnalytics.identify(userId);
export const retryMobileAppearanceAnalytics = () =>
  mobileAppearanceAnalytics.retry();
