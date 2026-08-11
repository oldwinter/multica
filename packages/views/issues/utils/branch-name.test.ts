import { describe, expect, it } from "vitest";
import { buildIssueBranchName } from "./branch-name";

describe("buildIssueBranchName", () => {
  it("combines the issue identifier with an ASCII-safe title slug", () => {
    expect(
      buildIssueBranchName("MUL-42", "Fix OAuth redirect"),
    ).toBe("mul-42-fix-oauth-redirect");
  });

  it("normalizes accents and removes apostrophes", () => {
    expect(
      buildIssueBranchName("MUL-7", "Can't reproduce café crash"),
    ).toBe("mul-7-cant-reproduce-cafe-crash");
  });

  it("falls back to the identifier when the title has no ASCII words", () => {
    expect(buildIssueBranchName("MUL-8", "修复登录跳转")).toBe("mul-8");
  });

  it("sanitizes custom characters in the issue identifier", () => {
    expect(buildIssueBranchName("ENG/WEB-1", "Fix OAuth redirect")).toBe(
      "eng-web-1-fix-oauth-redirect",
    );
  });

  it("caps long branch names without leaving a trailing separator", () => {
    expect(buildIssueBranchName("MUL-9", "a".repeat(100))).toBe(
      `mul-9-${"a".repeat(74)}`,
    );
  });

  it("caps the identifier-only fallback", () => {
    expect(buildIssueBranchName(`${"A".repeat(100)}-1`, "修复登录跳转")).toBe(
      "a".repeat(80),
    );
  });
});
