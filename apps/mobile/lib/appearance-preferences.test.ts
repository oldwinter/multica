import { describe, expect, it, vi } from "vitest";
import {
  readStoredAppearance,
  SKIN_STORAGE_KEY,
  THEME_STORAGE_KEY,
} from "./appearance-preferences";

describe("readStoredAppearance", () => {
  it("keeps a valid skin when the theme read fails and requests a retry", async () => {
    const readItem = vi.fn((key: string) => {
      if (key === THEME_STORAGE_KEY) return Promise.reject(new Error("locked"));
      return Promise.resolve(key === SKIN_STORAGE_KEY ? "relay" : null);
    });

    await expect(readStoredAppearance(readItem)).resolves.toEqual({
      preference: undefined,
      skin: "relay",
      shouldRetry: true,
    });
  });

  it("normalizes invalid fulfilled values without retrying", async () => {
    await expect(readStoredAppearance(async () => "unknown")).resolves.toEqual({
      preference: "system",
      skin: "tension",
      shouldRetry: false,
    });
  });
});
