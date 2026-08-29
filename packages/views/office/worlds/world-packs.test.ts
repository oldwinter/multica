// @vitest-environment node

import { describe, expect, it } from "vitest";
import { OFFICE_LIMITS, OFFICE_WORLD_IDS } from "@multica/core/office";
import {
  REQUIRED_MOTION_CLIPS,
  loadOfficeWorldPack,
} from "./world-packs";

describe("Office world packs", () => {
  it("implements the complete contract for exactly two worlds", async () => {
    expect(OFFICE_WORLD_IDS).toEqual(["studio", "expedition"]);

    for (const world of OFFICE_WORLD_IDS) {
      const pack = await loadOfficeWorldPack(world);

      expect(pack.id).toBe(world);
      expect(pack.contractVersion).toBe(1);
      expect(pack.layoutVersion).toBeGreaterThanOrEqual(1);
      expect(pack.map.layers.map((layer) => layer.name)).toEqual(
        expect.arrayContaining(["ground", "walk", "collision"]),
      );
      expect(pack.anchors.agentStations).toHaveLength(OFFICE_LIMITS.agents);
      expect(pack.anchors.squadBoards).toHaveLength(OFFICE_LIMITS.squads);
      expect(pack.anchors.activeIssues).toHaveLength(
        OFFICE_LIMITS.activeIssues,
      );
      expect(pack.anchors.dispatch).toHaveLength(1);
      expect(pack.anchors.overflow).toHaveLength(3);
      expect(pack.anchors.camera).toHaveLength(1);
      expect(Object.keys(pack.clips).sort()).toEqual(
        [...REQUIRED_MOTION_CLIPS].sort(),
      );
      for (const clip of Object.values(pack.clips)) {
        expect(clip.variants).toHaveLength(4);
        for (const frames of clip.variants) {
          expect(frames.length).toBeGreaterThanOrEqual(2);
        }
        expect(clip.fps).toBeGreaterThanOrEqual(8);
        expect(clip.fps).toBeLessThanOrEqual(16);
      }
      expect(new Set(pack.palette).size).toBeGreaterThanOrEqual(10);
      expect(pack.visuals.props.length).toBeGreaterThanOrEqual(6);
      expect(pack.visuals.decor.length).toBeGreaterThanOrEqual(48);
      expect(pack.visuals.backdropColor).toBeGreaterThanOrEqual(0);
      expect(pack.visuals.backdropColor).toBeLessThan(pack.palette.length);
      expect(pack.lighting.light.ambient).not.toBe(
        pack.lighting.dark.ambient,
      );
      expect(pack.hitRegions.length).toBeGreaterThan(0);
      expect(pack.assets.poster).toMatch(/\.png$/);
      expect(pack.assets.atlas).toMatch(/\.png$/);
      expect(pack.provenance).toMatch(/PROVENANCE\.json$/);
    }
  });

  it("uses materially different geometry, actors, props, stations, palettes, and motion", async () => {
    const [studio, expedition] = await Promise.all([
      loadOfficeWorldPack("studio"),
      loadOfficeWorldPack("expedition"),
    ]);

    expect(studio.map.width).not.toBe(expedition.map.width);
    expect(studio.visuals.actorSilhouette).not.toBe(
      expedition.visuals.actorSilhouette,
    );
    expect(studio.visuals.stationStyle).not.toBe(
      expedition.visuals.stationStyle,
    );
    expect(studio.visuals.props).not.toEqual(expedition.visuals.props);
    expect(studio.palette).not.toEqual(expedition.palette);
    expect(studio.clips.walk.variants).not.toEqual(
      expedition.clips.walk.variants,
    );
    expect(studio.visuals.decor).not.toEqual(expedition.visuals.decor);
  });
});
