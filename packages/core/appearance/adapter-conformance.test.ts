// @vitest-environment node

import { beforeEach, describe, expect, it } from "vitest";
import { defineAppearancePreferenceAdapterConformanceSuite } from "./conformance";
import type {
  AppearanceAdapterEvent,
  AppearanceEnvironment,
  AppearancePreferenceAdapter,
} from "./adapter";
import type { AppearancePreferences } from "./preferences";

function createMemoryHarness() {
  let stored: unknown | null = null;
  let applied: AppearancePreferences | null = null;
  const listeners = new Set<(event: AppearanceAdapterEvent) => void>();
  const environment: AppearanceEnvironment = {
    systemAppearance: "dark",
    reducedMotion: false,
    forcedColors: false,
    online: true,
  };

  const adapter: AppearancePreferenceAdapter = {
    source: "web",
    supportsRemoteSync: true,
    load: () => stored,
    persist: (preferences) => {
      stored = preferences;
    },
    apply: (preferences) => {
      applied = preferences;
    },
    getEnvironment: () => environment,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };

  return {
    create: () => adapter,
    seed: (value: unknown | null) => {
      stored = value;
    },
    readPersisted: () => stored,
    readApplied: () => applied,
    emit: (event: AppearanceAdapterEvent) => {
      for (const listener of listeners) listener(event);
    },
  };
}

defineAppearancePreferenceAdapterConformanceSuite(
  "memory web appearance adapter",
  "web",
  createMemoryHarness,
  { beforeEach, describe, expect, it },
);
