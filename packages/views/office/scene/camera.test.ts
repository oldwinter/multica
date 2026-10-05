// @vitest-environment node

import { describe, expect, it } from "vitest";
import {
  fitCamera,
  isWorldPointVisible,
  panCamera,
  zoomCameraAt,
} from "./camera";

describe("Office scene camera", () => {
  it("fits and centers a world with bounded integer-friendly scale", () => {
    const camera = fitCamera({
      viewport: { width: 800, height: 600 },
      world: { width: 1280, height: 768 },
    });

    expect(camera.x).toBeCloseTo(6);
    expect(camera.y).toBeCloseTo(63.6);
    expect(camera.scale).toBeCloseTo(0.615625);
  });

  it("keeps authored Office worlds dominant at wide and compact desktop sizes", () => {
    const cases = [
      {
        viewport: { width: 876, height: 840 },
        world: { width: 1152, height: 960 },
      },
      {
        viewport: { width: 769, height: 716 },
        world: { width: 1248, height: 1024 },
      },
    ];

    for (const input of cases) {
      const camera = fitCamera(input);
      const widthCoverage = (input.world.width * camera.scale) / input.viewport.width;
      const heightCoverage = (input.world.height * camera.scale) / input.viewport.height;

      expect(Math.max(widthCoverage, heightCoverage)).toBeCloseTo(0.985);
      expect(Math.min(widthCoverage, heightCoverage)).toBeGreaterThanOrEqual(0.75);
    }
  });

  it("zooms around the pointer while preserving its world coordinate", () => {
    const camera = { x: 100, y: 80, scale: 1 };
    const pointer = { x: 320, y: 240 };
    const before = {
      x: (pointer.x - camera.x) / camera.scale,
      y: (pointer.y - camera.y) / camera.scale,
    };
    const zoomed = zoomCameraAt(camera, pointer, 2);

    expect((pointer.x - zoomed.x) / zoomed.scale).toBe(before.x);
    expect((pointer.y - zoomed.y) / zoomed.scale).toBe(before.y);
    expect(zoomed.scale).toBe(2);
  });

  it("supports panning and viewport culling with a margin", () => {
    const camera = panCamera({ x: 10, y: 20, scale: 2 }, { x: -30, y: 15 });
    expect(camera).toEqual({ x: -20, y: 35, scale: 2 });
    expect(
      isWorldPointVisible({
        point: { x: 15, y: 10 },
        camera,
        viewport: { width: 100, height: 80 },
        margin: 8,
      }),
    ).toBe(true);
    expect(
      isWorldPointVisible({
        point: { x: 200, y: 200 },
        camera,
        viewport: { width: 100, height: 80 },
        margin: 8,
      }),
    ).toBe(false);
  });
});
