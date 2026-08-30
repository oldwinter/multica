import type { OfficeWorldId } from "@multica/core/office";
import {
  OFFICE_WORLD_CONTRACT_VERSION,
  REQUIRED_MOTION_CLIPS,
  type OfficeFrame,
  type OfficeDecorElement,
  type OfficeMotionClip,
  type OfficePoint,
  type OfficeWorldPack,
} from "./types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function record(value: unknown, path: string): Record<string, unknown> {
  if (!isRecord(value)) throw new Error(`${path} must be an object`);
  return value;
}

function string(value: unknown, path: string): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new Error(`${path} must be a non-empty string`);
  }
  return value;
}

function number(value: unknown, path: string): number {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error(`${path} must be a finite number`);
  }
  return value;
}

function boolean(value: unknown, path: string): boolean {
  if (typeof value !== "boolean") throw new Error(`${path} must be boolean`);
  return value;
}

function array(value: unknown, path: string): readonly unknown[] {
  if (!Array.isArray(value)) throw new Error(`${path} must be an array`);
  return value;
}

function strings(value: unknown, path: string): readonly string[] {
  return array(value, path).map((item, index) =>
    string(item, `${path}[${index}]`),
  );
}

function point(value: unknown, path: string): OfficePoint {
  const input = record(value, path);
  return {
    id: string(input.id, `${path}.id`),
    x: number(input.x, `${path}.x`),
    y: number(input.y, `${path}.y`),
  };
}

function points(value: unknown, path: string): readonly OfficePoint[] {
  return array(value, path).map((item, index) =>
    point(item, `${path}[${index}]`),
  );
}

function frame(value: unknown, path: string): OfficeFrame {
  const input = record(value, path);
  return {
    x: number(input.x, `${path}.x`),
    y: number(input.y, `${path}.y`),
    width: number(input.width, `${path}.width`),
    height: number(input.height, `${path}.height`),
  };
}

function frames(value: unknown): Readonly<Record<string, OfficeFrame>> {
  const input = record(value, "assets.frames");
  const output: Record<string, OfficeFrame> = {};
  for (const [key, candidate] of Object.entries(input)) {
    output[key] = frame(candidate, `assets.frames.${key}`);
  }
  return output;
}

function clip(value: unknown, path: string): OfficeMotionClip {
  const input = record(value, path);
  const variants = array(input.variants, `${path}.variants`).map(
    (variant, index) => strings(variant, `${path}.variants[${index}]`),
  );
  if (variants.length !== 4 || variants.some((frames) => frames.length < 2)) {
    throw new Error(`${path}.variants must define four two-frame variants`);
  }
  return {
    variants,
    fps: number(input.fps, `${path}.fps`),
    loop: boolean(input.loop, `${path}.loop`),
  };
}

function paletteIndex(value: unknown, path: string, paletteSize: number): number {
  const index = number(value, path);
  if (!Number.isInteger(index) || index < 0 || index >= paletteSize) {
    throw new Error(`${path} must be a palette index`);
  }
  return index;
}

function coordinateList(value: unknown, path: string): readonly number[] {
  const points = array(value, path).map((coordinate, index) =>
    number(coordinate, `${path}[${index}]`),
  );
  if (points.length < 4 || points.length % 2 !== 0) {
    throw new Error(`${path} must contain coordinate pairs`);
  }
  return points;
}

function decorElement(
  value: unknown,
  path: string,
  paletteSize: number,
): OfficeDecorElement {
  const input = record(value, path);
  const kind = string(input.kind, `${path}.kind`);
  const color = paletteIndex(input.color, `${path}.color`, paletteSize);
  switch (kind) {
    case "rect":
      return {
        kind,
        color,
        x: number(input.x, `${path}.x`),
        y: number(input.y, `${path}.y`),
        width: number(input.width, `${path}.width`),
        height: number(input.height, `${path}.height`),
      };
    case "circle":
      return {
        kind,
        color,
        x: number(input.x, `${path}.x`),
        y: number(input.y, `${path}.y`),
        radius: number(input.radius, `${path}.radius`),
      };
    case "polygon":
      return {
        kind,
        color,
        points: coordinateList(input.points, `${path}.points`),
      };
    case "line":
      return {
        kind,
        color,
        points: coordinateList(input.points, `${path}.points`),
        width: number(input.width, `${path}.width`),
      };
    default:
      throw new Error(`${path}.kind is invalid`);
  }
}

function worldId(value: unknown): OfficeWorldId {
  if (value === "studio" || value === "expedition") return value;
  throw new Error("id must be a supported Office world");
}

export interface OfficeWorldAssetUrls {
  readonly map: string;
  readonly atlas: string;
  readonly poster: string;
  readonly provenance: string;
}

export function parseOfficeWorldPack(
  value: unknown,
  urls: OfficeWorldAssetUrls,
): OfficeWorldPack {
  const input = record(value, "world pack");
  const map = record(input.map, "map");
  const assets = record(input.assets, "assets");
  const atlasSize = record(assets.atlasSize, "assets.atlasSize");
  const posterSize = record(assets.posterSize, "assets.posterSize");
  const anchors = record(input.anchors, "anchors");
  const clips = record(input.clips, "clips");
  const lighting = record(input.lighting, "lighting");
  const light = record(lighting.light, "lighting.light");
  const dark = record(lighting.dark, "lighting.dark");
  const visuals = record(input.visuals, "visuals");
  const palette = strings(input.palette, "palette");
  string(map.asset, "map.asset");
  string(assets.atlas, "assets.atlas");
  string(assets.poster, "assets.poster");
  string(input.provenance, "provenance");
  const contractVersion = number(input.contractVersion, "contractVersion");
  if (contractVersion !== OFFICE_WORLD_CONTRACT_VERSION) {
    throw new Error(`unsupported Office world contract ${contractVersion}`);
  }

  const parsedClips = {
    idle: clip(clips.idle, "clips.idle"),
    wait: clip(clips.wait, "clips.wait"),
    work: clip(clips.work, "clips.work"),
    walk: clip(clips.walk, "clips.walk"),
    unstable: clip(clips.unstable, "clips.unstable"),
    offline: clip(clips.offline, "clips.offline"),
    completion: clip(clips.completion, "clips.completion"),
    failure: clip(clips.failure, "clips.failure"),
  } satisfies Record<(typeof REQUIRED_MOTION_CLIPS)[number], OfficeMotionClip>;

  return {
    id: worldId(input.id),
    contractVersion: OFFICE_WORLD_CONTRACT_VERSION,
    layoutVersion: number(input.layoutVersion, "layoutVersion"),
    map: {
      asset: urls.map,
      width: number(map.width, "map.width"),
      height: number(map.height, "map.height"),
      tileSize: number(map.tileSize, "map.tileSize"),
      layers: array(map.layers, "map.layers").map((layer, index) => {
        const item = record(layer, `map.layers[${index}]`);
        return {
          name: string(item.name, `map.layers[${index}].name`),
          type: string(item.type, `map.layers[${index}].type`),
        };
      }),
    },
    assets: {
      atlas: urls.atlas,
      poster: urls.poster,
      atlasSize: {
        width: number(atlasSize.width, "assets.atlasSize.width"),
        height: number(atlasSize.height, "assets.atlasSize.height"),
      },
      posterSize: {
        width: number(posterSize.width, "assets.posterSize.width"),
        height: number(posterSize.height, "assets.posterSize.height"),
      },
      frames: frames(assets.frames),
    },
    anchors: {
      agentStations: points(anchors.agentStations, "anchors.agentStations"),
      squadBoards: points(anchors.squadBoards, "anchors.squadBoards"),
      activeIssues: points(anchors.activeIssues, "anchors.activeIssues"),
      dispatch: points(anchors.dispatch, "anchors.dispatch"),
      overflow: points(anchors.overflow, "anchors.overflow"),
      camera: points(anchors.camera, "anchors.camera"),
    },
    clips: parsedClips,
    palette,
    lighting: {
      light: {
        ambient: string(light.ambient, "lighting.light.ambient"),
        overlayAlpha: number(
          light.overlayAlpha,
          "lighting.light.overlayAlpha",
        ),
      },
      dark: {
        ambient: string(dark.ambient, "lighting.dark.ambient"),
        overlayAlpha: number(
          dark.overlayAlpha,
          "lighting.dark.overlayAlpha",
        ),
      },
    },
    visuals: {
      actorSilhouette: string(
        visuals.actorSilhouette,
        "visuals.actorSilhouette",
      ),
      stationStyle: string(visuals.stationStyle, "visuals.stationStyle"),
      props: strings(visuals.props, "visuals.props"),
      backdropColor: paletteIndex(
        visuals.backdropColor,
        "visuals.backdropColor",
        palette.length,
      ),
      decor: array(visuals.decor, "visuals.decor").map((element, index) =>
        decorElement(element, `visuals.decor[${index}]`, palette.length),
      ),
    },
    hitRegions: array(input.hitRegions, "hitRegions").map((region, index) => {
      const item = record(region, `hitRegions[${index}]`);
      const role = string(item.role, `hitRegions[${index}].role`);
      if (role !== "agent" && role !== "squad" && role !== "issue") {
        throw new Error(`hitRegions[${index}].role is invalid`);
      }
      return {
        role,
        polygon: array(item.polygon, `hitRegions[${index}].polygon`).map(
          (coordinate, coordinateIndex) =>
            number(
              coordinate,
              `hitRegions[${index}].polygon[${coordinateIndex}]`,
            ),
        ),
      };
    }),
    provenance: urls.provenance,
  };
}
