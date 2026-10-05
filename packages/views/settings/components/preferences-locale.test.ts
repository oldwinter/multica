// @vitest-environment node

import { describe, expect, it } from "vitest";
import { resolveSettingsLocale } from "./preferences-locale";

describe("resolveSettingsLocale", () => {
  it.each([
    ["en", "en"],
    ["zh-Hans", "zh-Hans"],
    ["zh-Hans-CN", "zh-Hans"],
    ["ko-KR", "ko"],
    ["ja-JP", "ja"],
  ])("maps %s to %s", (language, expected) => {
    expect(resolveSettingsLocale(language)).toBe(expected);
  });

  it("uses the default for an unsupported language", () => {
    expect(resolveSettingsLocale("xx-XX")).toBe("en");
  });
});
