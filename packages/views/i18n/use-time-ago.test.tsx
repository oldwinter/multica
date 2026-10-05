// @vitest-environment jsdom

import { cleanup, renderHook } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import enCommon from "../locales/en/common.json";
import zhHansCommon from "../locales/zh-Hans/common.json";
import { useTimeAgo, useTimeUntil } from "./use-time-ago";

const NOW = new Date("2026-08-31T12:00:00.000Z");

function createWrapper(locale: "en" | "zh-Hans" = "en") {
  const resources = {
    en: { common: enCommon },
    "zh-Hans": { common: zhHansCommon },
  };

  return function Wrapper({ children }: { readonly children: ReactNode }) {
    return (
      <I18nProvider locale={locale} resources={resources}>
        {children}
      </I18nProvider>
    );
  };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("relative time hooks", () => {
  it("formats future times with countdown copy instead of just-now copy", () => {
    const { result } = renderHook(() => useTimeUntil(), {
      wrapper: createWrapper(),
    });

    expect(result.current("2026-08-31T12:05:00.000Z")).toBe("in 5m");
    expect(result.current("2026-08-31T13:00:00.000Z")).toBe("in 1h");
  });

  it("localizes future times while keeping past time-ago output intact", () => {
    const future = renderHook(() => useTimeUntil(), {
      wrapper: createWrapper("zh-Hans"),
    });
    const past = renderHook(() => useTimeAgo(), {
      wrapper: createWrapper("zh-Hans"),
    });

    expect(future.result.current("2026-08-31T12:05:00.000Z")).toBe("5 分钟后");
    expect(past.result.current("2026-08-31T11:55:00.000Z")).toBe("5 分钟前");
  });
});
