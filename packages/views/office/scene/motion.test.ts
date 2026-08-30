// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { OfficeAgentSceneState } from "./contracts";
import {
  OFFICE_ACTOR_HOME,
  sampleOfficeAgentMotion,
} from "./motion";

function state(
  overrides: Partial<OfficeAgentSceneState> = {},
): OfficeAgentSceneState {
  return {
    availability: "online",
    workload: "idle",
    runningCount: 0,
    queuedCount: 0,
    clip: "idle",
    stationLit: false,
    ambientMotion: false,
    pulse: false,
    flicker: false,
    ...overrides,
  };
}

describe("Office station-local motion", () => {
  it("is deterministic per entity and stays within two integer pixels", () => {
    const elapsedTimes = [0, 250, 500, 750, 1_000, 1_250, 1_500];
    const inputState = state({ ambientMotion: true });
    const samples = elapsedTimes.map((elapsedMs) =>
      sampleOfficeAgentMotion({
        entityKey: "agent:alpha",
        elapsedMs,
        motionDisabled: false,
        state: inputState,
      }),
    );
    const repeated = elapsedTimes.map((elapsedMs) =>
      sampleOfficeAgentMotion({
        entityKey: "agent:alpha",
        elapsedMs,
        motionDisabled: false,
        state: inputState,
      }),
    );
    const otherEntity = elapsedTimes.map((elapsedMs) =>
      sampleOfficeAgentMotion({
        entityKey: "agent:beta",
        elapsedMs,
        motionDisabled: false,
        state: inputState,
      }),
    );

    expect(samples).toEqual(repeated);
    expect(samples).not.toEqual(otherEntity);
    expect(
      samples.some(
        (sample) =>
          sample.x !== OFFICE_ACTOR_HOME.x ||
          sample.y !== OFFICE_ACTOR_HOME.y,
      ),
    ).toBe(true);
    for (const sample of samples) {
      expect(Number.isInteger(sample.x)).toBe(true);
      expect(Number.isInteger(sample.y)).toBe(true);
      expect(Math.abs(sample.x - OFFICE_ACTOR_HOME.x)).toBeLessThanOrEqual(2);
      expect(Math.abs(sample.y - OFFICE_ACTOR_HOME.y)).toBeLessThanOrEqual(2);
      expect(sample.scale).toBe(1.08);
      expect(sample.alphaMultiplier).toBe(1);
    }
  });

  it("applies restrained deterministic pulse and flicker modulation", () => {
    const inputState = state({ pulse: true, flicker: true });
    const samples = Array.from({ length: 24 }, (_, index) =>
      sampleOfficeAgentMotion({
        entityKey: "agent:unstable",
        elapsedMs: index * 137,
        motionDisabled: false,
        state: inputState,
      }),
    );

    expect(samples.some((sample) => sample.scale > 1.08)).toBe(true);
    expect(samples.some((sample) => sample.alphaMultiplier < 1)).toBe(true);
    for (const sample of samples) {
      expect(sample.scale).toBeGreaterThanOrEqual(1.08);
      expect(sample.scale).toBeLessThanOrEqual(1.1);
      expect(sample.alphaMultiplier).toBeGreaterThanOrEqual(0.88);
      expect(sample.alphaMultiplier).toBeLessThanOrEqual(1);
    }
  });

  it("returns the exact authored anchor and neutral modulation when frozen", () => {
    const inputState = state({
      ambientMotion: true,
      pulse: true,
      flicker: true,
    });

    for (const elapsedMs of [0, 777, 10_000]) {
      expect(
        sampleOfficeAgentMotion({
          entityKey: "agent:frozen",
          elapsedMs,
          motionDisabled: true,
          state: inputState,
        }),
      ).toEqual({
        x: OFFICE_ACTOR_HOME.x,
        y: OFFICE_ACTOR_HOME.y,
        scale: 1.08,
        alphaMultiplier: 1,
      });
    }
  });
});
