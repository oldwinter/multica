// @vitest-environment node

import { describe, expect, it, vi } from "vitest";
import {
  SEMANTIC_TOKEN_CONTRACT_VERSION,
  SEMANTIC_TOKEN_ROLES,
} from "@multica/core/constants/semantic-token-schema";

import {
  MOBILE_SEMANTIC_TOKEN_CONTRACT_VERSION,
  THEMES,
  validateMobileThemeContract,
} from "./theme";

vi.mock("@react-navigation/native", () => ({
  DarkTheme: { dark: true, colors: {} },
  DefaultTheme: { dark: false, colors: {} },
}));

describe("mobile semantic theme contract", () => {
  it("implements every platform-neutral role for all six palettes", () => {
    expect(MOBILE_SEMANTIC_TOKEN_CONTRACT_VERSION).toBe(SEMANTIC_TOKEN_CONTRACT_VERSION);

    for (const [skin, modes] of Object.entries(THEMES)) {
      for (const [mode, palette] of Object.entries(modes)) {
        expect(validateMobileThemeContract(palette), `${skin}/${mode}`).toEqual([]);
      }
    }
  });

  it("reports every role affected by a missing palette value", () => {
    const broken = { ...THEMES.tension.light, info: "" };

    expect(validateMobileThemeContract(broken)).toEqual([
      expect.objectContaining({ role: "info" }),
      expect.objectContaining({ role: "statusInProgress" }),
    ]);
    expect(SEMANTIC_TOKEN_ROLES).toContain("statusInProgress");
  });
});
