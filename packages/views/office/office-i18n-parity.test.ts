// @vitest-environment node
import { describe, expect, it } from "vitest";
import en from "../locales/en/office.json";
import zhHans from "../locales/zh-Hans/office.json";
import ja from "../locales/ja/office.json";
import ko from "../locales/ko/office.json";

const LOCALES: Record<string, unknown> = {
  en,
  "zh-Hans": zhHans,
  ja,
  ko,
};

function leafEntries(value: unknown, prefix = ""): readonly string[] {
  if (typeof value === "string") return [`${prefix}=${value}`];
  if (typeof value !== "object" || value === null) return [];
  return Object.entries(value).flatMap(([key, child]) =>
    leafEntries(child, prefix ? `${prefix}.${key}` : key),
  );
}

function leafKeys(value: unknown): readonly string[] {
  return leafEntries(value)
    .map((entry) => entry.slice(0, entry.indexOf("=")))
    .sort();
}

describe("Office locale parity", () => {
  it("keeps the same non-empty key set in all four locales", () => {
    const englishKeys = leafKeys(en);
    for (const [locale, resources] of Object.entries(LOCALES)) {
      expect(leafKeys(resources), `${locale} key set`).toEqual(englishKeys);
      for (const entry of leafEntries(resources)) {
        const value = entry.slice(entry.indexOf("=") + 1);
        expect(value.trim().length, `${locale}: ${entry}`).toBeGreaterThan(0);
      }
    }
  });
});
