// @vitest-environment node

import { describe, expect, it } from "vitest";
import { loadOfficeWorldPack } from "../worlds/world-packs";
import { PlacementRegistry, placeCohort } from "./placement";

describe("deterministic Office placement", () => {
  it("uses world, layout version, kind, and entity id instead of list order", async () => {
    const pack = await loadOfficeWorldPack("studio");
    const ids = Array.from({ length: 20 }, (_, index) => `agent-${index}`);

    const forward = placeCohort({
      world: pack.id,
      layoutVersion: pack.layoutVersion,
      kind: "agent",
      ids,
      anchors: pack.anchors.agentStations,
    });
    const reversed = placeCohort({
      world: pack.id,
      layoutVersion: pack.layoutVersion,
      kind: "agent",
      ids: [...ids].reverse(),
      anchors: pack.anchors.agentStations,
    });

    expect([...forward.placements]).toEqual([...reversed.placements]);
    expect(new Set(forward.placements.values()).size).toBe(ids.length);
  });

  it("keeps existing allocations fixed while a mount adds and removes entities", async () => {
    const pack = await loadOfficeWorldPack("studio");
    const registry = new PlacementRegistry({
      world: pack.id,
      layoutVersion: pack.layoutVersion,
    });
    const first = registry.reconcile(
      "agent",
      ["beta", "gamma"],
      pack.anchors.agentStations,
    );
    const second = registry.reconcile(
      "agent",
      ["alpha", "beta", "gamma"],
      pack.anchors.agentStations,
    );
    const third = registry.reconcile(
      "agent",
      ["alpha", "gamma"],
      pack.anchors.agentStations,
    );

    expect(second.placements.get("beta")).toBe(first.placements.get("beta"));
    expect(second.placements.get("gamma")).toBe(first.placements.get("gamma"));
    expect(third.placements.get("gamma")).toBe(first.placements.get("gamma"));
  });

  it.each([
    ["agent", 40],
    ["squad", 12],
    ["issue", 48],
  ] as const)("caps %s cohorts with exact overflow", async (kind, capacity) => {
    const pack = await loadOfficeWorldPack("expedition");
    const anchors =
      kind === "agent"
        ? pack.anchors.agentStations
        : kind === "squad"
          ? pack.anchors.squadBoards
          : pack.anchors.activeIssues;
    const result = placeCohort({
      world: pack.id,
      layoutVersion: pack.layoutVersion,
      kind,
      ids: Array.from({ length: capacity + 7 }, (_, index) => `${kind}-${index}`),
      anchors,
    });

    expect(result.placements.size).toBe(capacity);
    expect(result.overflow).toBe(7);
  });
});
