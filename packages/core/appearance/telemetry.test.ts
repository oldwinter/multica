// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  createAppearanceDiagnostics,
  serializeAppearanceDiagnostics,
  type AppearanceDiagnosticsSnapshot,
} from "./diagnostics";
import {
  createAppearanceAnalyticsEvent,
  type AppearanceAnalyticsCapture,
} from "./telemetry";
import {
  APPEARANCE_PREFERENCES_VERSION,
  APPEARANCE_TOKEN_CONTRACT_VERSION,
  type AppearancePreferences,
} from "./preferences";

const preferences: AppearancePreferences = {
  version: APPEARANCE_PREFERENCES_VERSION,
  tokenContractVersion: APPEARANCE_TOKEN_CONTRACT_VERSION,
  skin: "relay",
  requestedAppearance: "system",
  resolvedAppearance: "dark",
  source: "server",
  updatedAt: "2026-08-23T10:00:00.000Z",
  syncState: { status: "failed", errorClass: "network" },
};

describe("appearance diagnostics", () => {
  it("exposes only bounded state needed to diagnose appearance behavior", () => {
    const snapshot: AppearanceDiagnosticsSnapshot = createAppearanceDiagnostics(
      preferences,
      {
        adapterSource: "desktop",
        reducedMotion: true,
        forcedColors: false,
        recoveredFields: ["skin", "updatedAt"],
      },
    );

    expect(snapshot).toEqual({
      preferenceVersion: 1,
      tokenContractVersion: 1,
      skin: "relay",
      requestedAppearance: "system",
      resolvedAppearance: "dark",
      preferenceSource: "server",
      adapterSource: "desktop",
      syncStatus: "failed",
      lastSyncErrorClass: "network",
      reducedMotion: true,
      forcedColors: false,
      recoveredFields: ["skin", "updatedAt"],
    });
    expect(serializeAppearanceDiagnostics(snapshot)).toBe(
      JSON.stringify(snapshot, null, 2),
    );
    expect(serializeAppearanceDiagnostics(snapshot)).not.toContain(
      "2026-08-23",
    );
  });
});

describe("appearance analytics types", () => {
  it("constructs a bounded event without accepting arbitrary properties", () => {
    expect(
      createAppearanceAnalyticsEvent("skin_selected", {
        skin: "field",
        previousSkin: "tension",
        adapterSource: "web",
      }),
    ).toEqual({
      name: "skin_selected",
      properties: {
        skin: "field",
        previousSkin: "tension",
        adapterSource: "web",
      },
    });

    const capture: AppearanceAnalyticsCapture = () => undefined;
    capture("appearance_viewed", {
      skin: "tension",
      requestedAppearance: "system",
      resolvedAppearance: "dark",
      adapterSource: "mobile",
    });

    // @ts-expect-error appearance analytics never accepts user content
    capture("appearance_viewed", { skin: "tension", requestedAppearance: "system", resolvedAppearance: "dark", adapterSource: "web", content: "secret" });
    // @ts-expect-error appearance analytics never accepts route data
    createAppearanceAnalyticsEvent("reset", { adapterSource: "web", route: "/private" });
    // @ts-expect-error appearance analytics never accepts workspace identifiers
    capture("sync_failed", { adapterSource: "desktop", errorClass: "network", workspaceId: "ws-secret" });

    expect(true).toBe(true);
  });
});
