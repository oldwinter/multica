// @vitest-environment node

import { describe, expect, it, vi } from "vitest";
import {
  AppConfigSchema,
  EMPTY_APP_CONFIG,
} from "@multica/core/api/schemas";
import { parseWithFallback } from "@/lib/parse-response";
import { createMobileAppearanceAnalyticsTransport } from "./appearance-analytics";

const config = {
  posthog_key: "ph-test",
  posthog_host: "https://analytics.example.test/",
  analytics_environment: "test",
};

describe("mobile appearance analytics transport", () => {
  it("buffers until configured and sends only bounded event properties", async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true });
    const analytics = createMobileAppearanceAnalyticsTransport(fetchImpl);
    analytics.identify("user-1");
    analytics.capture("skin_selected", {
      skin: "field",
      previousSkin: "tension",
      adapterSource: "mobile",
    });

    expect(analytics.pendingCount()).toBe(1);
    analytics.configure(config);
    await analytics.retry();

    expect(fetchImpl).toHaveBeenCalledOnce();
    const [url, init] = fetchImpl.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("https://analytics.example.test/batch/");
    expect(init.signal).toBeInstanceOf(AbortSignal);
    expect(JSON.parse(String(init.body))).toEqual({
      api_key: "ph-test",
      batch: [
        {
          event: "skin_selected",
          properties: {
            skin: "field",
            previousSkin: "tension",
            adapterSource: "mobile",
            distinct_id: "user-1",
            client_type: "mobile",
            event_schema_version: 1,
            environment: "test",
          },
        },
      ],
    });
  });

  it("drains events captured while a batch is in flight", async () => {
    let resolveFirst!: (value: { ok: boolean }) => void;
    const fetchImpl = vi
      .fn()
      .mockReturnValueOnce(
        new Promise<{ ok: boolean }>((resolve) => {
          resolveFirst = resolve;
        }),
      )
      .mockResolvedValue({ ok: true });
    const analytics = createMobileAppearanceAnalyticsTransport(fetchImpl);
    analytics.configure(config);
    analytics.capture("reset", { adapterSource: "mobile" });
    analytics.capture("appearance_selected", {
      appearance: "dark",
      previousAppearance: "system",
      adapterSource: "mobile",
    });

    const draining = analytics.retry();
    resolveFirst({ ok: true });
    await draining;

    expect(fetchImpl).toHaveBeenCalledTimes(2);
    expect(analytics.pendingCount()).toBe(0);
  });

  it("aborts a suspended batch and releases it for retry", async () => {
    vi.useFakeTimers();
    try {
      const fetchImpl = vi.fn(
        (_input: string, init: RequestInit) =>
          new Promise<{ ok: boolean }>((_resolve, reject) => {
            init.signal?.addEventListener("abort", () =>
              reject(new Error("aborted")),
            );
          }),
      );
      const analytics = createMobileAppearanceAnalyticsTransport(fetchImpl, 50);
      analytics.configure(config);
      analytics.capture("reset", { adapterSource: "mobile" });

      await vi.advanceTimersByTimeAsync(50);
      expect(analytics.pendingCount()).toBe(1);
      expect(fetchImpl.mock.calls[0]?.[1].signal?.aborted).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });

  it("uses the shared config fallback for malformed endpoint responses", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    const parsed = parseWithFallback(
      { cdn_domain: 42, posthog_key: ["not-a-key"] },
      AppConfigSchema,
      EMPTY_APP_CONFIG,
      { endpoint: "getAppearanceAnalyticsConfig" },
    );

    expect(parsed).toBe(EMPTY_APP_CONFIG);
    expect(parsed.posthog_key).toBeUndefined();
    expect(warn).toHaveBeenCalledOnce();
    warn.mockRestore();
  });

  it("keeps a bounded retry queue after network failure", async () => {
    let resolveFirst!: (value: { ok: boolean }) => void;
    const firstResponse = new Promise<{ ok: boolean }>((resolve) => {
      resolveFirst = resolve;
    });
    const fetchImpl = vi
      .fn()
      .mockReturnValueOnce(firstResponse)
      .mockResolvedValueOnce({ ok: true });
    const analytics = createMobileAppearanceAnalyticsTransport(fetchImpl);
    analytics.configure(config);
    analytics.capture("reset", { adapterSource: "mobile" });
    resolveFirst({ ok: false });
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(analytics.pendingCount()).toBe(1);
    await analytics.retry();
    expect(analytics.pendingCount()).toBe(0);
    expect(fetchImpl).toHaveBeenCalledTimes(2);
  });

  it("drops buffered events when analytics is disabled", async () => {
    const fetchImpl = vi.fn().mockResolvedValue({ ok: true });
    const analytics = createMobileAppearanceAnalyticsTransport(fetchImpl);
    analytics.capture("reset", { adapterSource: "mobile" });
    analytics.configure({ ...config, posthog_key: "" });
    await analytics.retry();

    expect(analytics.pendingCount()).toBe(0);
    expect(fetchImpl).not.toHaveBeenCalled();
  });
});
