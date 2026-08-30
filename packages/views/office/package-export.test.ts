// @vitest-environment node

import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("@multica/views Office export", () => {
  it("exports the shared Office leaf", () => {
    const packageJson = JSON.parse(
      readFileSync(new URL("../package.json", import.meta.url), "utf8"),
    ) as { exports?: Record<string, string> };

    expect(packageJson.exports?.["./office"]).toBe("./office/index.ts");
  });
});
