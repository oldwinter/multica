import type { OfficeWorldId } from "@multica/core/office";
import type { OfficePoint } from "../worlds/types";

export type OfficePlacementKind = "agent" | "squad" | "issue";

interface PlacementInput {
  readonly world: OfficeWorldId;
  readonly layoutVersion: number;
  readonly kind: OfficePlacementKind;
  readonly ids: readonly string[];
  readonly anchors: readonly OfficePoint[];
}

export interface PlacementResult {
  readonly placements: ReadonlyMap<string, OfficePoint>;
  readonly overflow: number;
}

function stableHash(value: string): number {
  let hash = 0x811c9dc5;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

function seedOf(input: {
  readonly world: OfficeWorldId;
  readonly layoutVersion: number;
  readonly kind: OfficePlacementKind;
}): string {
  return `${input.world}|${input.layoutVersion}|${input.kind}`;
}

function sortedUniqueIds(ids: readonly string[], seed: string): readonly string[] {
  return [...new Set(ids)].sort((left, right) => {
    const score = stableHash(`${seed}|subject|${left}`) - stableHash(`${seed}|subject|${right}`);
    return score || left.localeCompare(right);
  });
}

function rankedAnchors(
  id: string,
  anchors: readonly OfficePoint[],
  seed: string,
): readonly OfficePoint[] {
  return [...anchors].sort((left, right) => {
    const score =
      stableHash(`${seed}|${id}|${right.id}`) -
      stableHash(`${seed}|${id}|${left.id}`);
    return score || left.id.localeCompare(right.id);
  });
}

function sortedPlacements(
  placements: ReadonlyMap<string, OfficePoint>,
): ReadonlyMap<string, OfficePoint> {
  return new Map(
    [...placements].sort(([left], [right]) => left.localeCompare(right)),
  );
}

export function placeCohort(input: PlacementInput): PlacementResult {
  const seed = seedOf(input);
  const ids = sortedUniqueIds(input.ids, seed);
  const occupied = new Set<string>();
  const placements = new Map<string, OfficePoint>();
  for (const id of ids) {
    const anchor = rankedAnchors(id, input.anchors, seed).find(
      (candidate) => !occupied.has(candidate.id),
    );
    if (!anchor) continue;
    placements.set(id, anchor);
    occupied.add(anchor.id);
  }
  return {
    placements: sortedPlacements(placements),
    overflow: Math.max(0, ids.length - placements.size),
  };
}

export class PlacementRegistry {
  readonly #world: OfficeWorldId;
  readonly #layoutVersion: number;
  readonly #placements = new Map<
    OfficePlacementKind,
    Map<string, OfficePoint>
  >();

  constructor(input: {
    readonly world: OfficeWorldId;
    readonly layoutVersion: number;
  }) {
    this.#world = input.world;
    this.#layoutVersion = input.layoutVersion;
  }

  reconcile(
    kind: OfficePlacementKind,
    rawIds: readonly string[],
    anchors: readonly OfficePoint[],
  ): PlacementResult {
    const seed = seedOf({
      world: this.#world,
      layoutVersion: this.#layoutVersion,
      kind,
    });
    const ids = sortedUniqueIds(rawIds, seed);
    const idSet = new Set(ids);
    const anchorsById = new Map(anchors.map((anchor) => [anchor.id, anchor]));
    const current = this.#placements.get(kind) ?? new Map<string, OfficePoint>();

    for (const [id, anchor] of current) {
      if (!idSet.has(id) || !anchorsById.has(anchor.id)) current.delete(id);
    }

    const occupied = new Set([...current.values()].map((anchor) => anchor.id));
    for (const id of ids) {
      if (current.has(id)) continue;
      const anchor = rankedAnchors(id, anchors, seed).find(
        (candidate) => !occupied.has(candidate.id),
      );
      if (!anchor) continue;
      current.set(id, anchor);
      occupied.add(anchor.id);
    }
    this.#placements.set(kind, current);
    return {
      placements: sortedPlacements(current),
      overflow: Math.max(0, ids.length - current.size),
    };
  }
}
