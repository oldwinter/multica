#!/usr/bin/env node

import { createHash } from "node:crypto";
import {
  existsSync,
  readFileSync,
  readdirSync,
  statSync,
} from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { inflateSync } from "node:zlib";

const expectedWorlds = ["expedition", "studio"];
const requiredClips = [
  "completion",
  "failure",
  "idle",
  "offline",
  "unstable",
  "wait",
  "walk",
  "work",
];
const capacities = {
  agentStations: 40,
  squadBoards: 12,
  activeIssues: 48,
  dispatch: 1,
  overflow: 3,
  camera: 1,
};
const transferBudget = 2 * 1024 * 1024;
const decodedBudget = 16 * 1024 * 1024;

function fail(message) {
  throw new Error(`Office asset validation failed: ${message}`);
}

function object(value, path) {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    fail(`${path} must be an object`);
  }
  return value;
}

function array(value, path) {
  if (!Array.isArray(value)) fail(`${path} must be an array`);
  return value;
}

function string(value, path) {
  if (typeof value !== "string" || value.length === 0) {
    fail(`${path} must be a non-empty string`);
  }
  return value;
}

function finiteNumber(value, path) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    fail(`${path} must be a finite number`);
  }
  return value;
}

function readJson(path) {
  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    fail(`${path} is not valid JSON: ${error instanceof Error ? error.message : "unknown error"}`);
  }
}

function sha256(path) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

function resolveInside(base, candidate, label) {
  const path = resolve(base, string(candidate, label));
  const rel = relative(base, path);
  if (rel === "" || rel.startsWith("..") || resolve(base, rel) !== path) {
    fail(`${label} escapes its world directory`);
  }
  if (!statSync(path, { throwIfNoEntry: false })?.isFile()) {
    fail(`${label} does not exist: ${rel}`);
  }
  return path;
}

function pngDimensions(path) {
  const file = readFileSync(path);
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
  if (file.length < 24 || !file.subarray(0, 8).equals(signature)) {
    fail(`${relative(process.cwd(), path)} is not a PNG`);
  }
  return { width: file.readUInt32BE(16), height: file.readUInt32BE(20) };
}

function pngPixels(path) {
  const file = readFileSync(path);
  const dimensions = pngDimensions(path);
  if (file[24] !== 8 || file[25] !== 6) {
    fail(`${relative(process.cwd(), path)} must be an 8-bit RGBA PNG`);
  }
  const chunks = [];
  let offset = 8;
  while (offset + 12 <= file.length) {
    const length = file.readUInt32BE(offset);
    const type = file.toString("ascii", offset + 4, offset + 8);
    const start = offset + 8;
    const end = start + length;
    if (end + 4 > file.length) fail(`${path} has a truncated PNG chunk`);
    if (type === "IDAT") chunks.push(file.subarray(start, end));
    offset = end + 4;
    if (type === "IEND") break;
  }
  const decoded = inflateSync(Buffer.concat(chunks));
  const stride = dimensions.width * 4;
  if (decoded.length !== dimensions.height * (stride + 1)) {
    fail(`${relative(process.cwd(), path)} has an unexpected pixel payload`);
  }
  const pixels = Buffer.alloc(dimensions.width * dimensions.height * 4);
  for (let y = 0; y < dimensions.height; y += 1) {
    const source = y * (stride + 1);
    if (decoded[source] !== 0) {
      fail(`${relative(process.cwd(), path)} must use deterministic PNG filter 0`);
    }
    decoded.copy(pixels, y * stride, source + 1, source + stride + 1);
  }
  return { ...dimensions, pixels };
}

function pixelDifferenceRatio(left, right) {
  if (left.width !== right.width || left.height !== right.height) return 1;
  let different = 0;
  for (let offset = 0; offset < left.pixels.length; offset += 4) {
    const distance =
      Math.abs(left.pixels[offset] - right.pixels[offset]) +
      Math.abs(left.pixels[offset + 1] - right.pixels[offset + 1]) +
      Math.abs(left.pixels[offset + 2] - right.pixels[offset + 2]) +
      Math.abs(left.pixels[offset + 3] - right.pixels[offset + 3]);
    if (distance >= 48) different += 1;
  }
  return different / (left.pixels.length / 4);
}

function sameDimensions(actual, expected, path) {
  const input = object(expected, path);
  if (
    actual.width !== finiteNumber(input.width, `${path}.width`) ||
    actual.height !== finiteNumber(input.height, `${path}.height`)
  ) {
    fail(
      `${path} expected ${input.width}x${input.height}, got ${actual.width}x${actual.height}`,
    );
  }
}

function validateCoordinatePairs(value, path, bounds) {
  const points = array(value, path);
  if (points.length < 4 || points.length % 2 !== 0) {
    fail(`${path} must contain coordinate pairs`);
  }
  for (let index = 0; index < points.length; index += 2) {
    const x = finiteNumber(points[index], `${path}[${index}]`);
    const y = finiteNumber(points[index + 1], `${path}[${index + 1}]`);
    if (x < 0 || y < 0 || x > bounds.width || y > bounds.height) {
      fail(`${path} contains a point outside map bounds`);
    }
  }
  return points;
}

function validateDecor(value, paletteSize, bounds, world) {
  const decor = array(value, `${world}.visuals.decor`);
  for (const [index, candidate] of decor.entries()) {
    const path = `${world}.visuals.decor[${index}]`;
    const element = object(candidate, path);
    const kind = string(element.kind, `${path}.kind`);
    const color = finiteNumber(element.color, `${path}.color`);
    if (!Number.isInteger(color) || color < 0 || color >= paletteSize) {
      fail(`${path}.color must be a palette index`);
    }
    if (kind === "rect") {
      const x = finiteNumber(element.x, `${path}.x`);
      const y = finiteNumber(element.y, `${path}.y`);
      const width = finiteNumber(element.width, `${path}.width`);
      const height = finiteNumber(element.height, `${path}.height`);
      if (
        width <= 0 ||
        height <= 0 ||
        x < 0 ||
        y < 0 ||
        x + width > bounds.width ||
        y + height > bounds.height
      ) {
        fail(`${path} must be a positive rectangle inside map bounds`);
      }
    } else if (kind === "circle") {
      const x = finiteNumber(element.x, `${path}.x`);
      const y = finiteNumber(element.y, `${path}.y`);
      const radius = finiteNumber(element.radius, `${path}.radius`);
      if (
        radius <= 0 ||
        x - radius < 0 ||
        y - radius < 0 ||
        x + radius > bounds.width ||
        y + radius > bounds.height
      ) {
        fail(`${path} must be a positive circle inside map bounds`);
      }
    } else if (kind === "polygon") {
      const points = validateCoordinatePairs(element.points, `${path}.points`, bounds);
      if (points.length < 6) fail(`${path}.points must define a polygon`);
    } else if (kind === "line") {
      validateCoordinatePairs(element.points, `${path}.points`, bounds);
      if (finiteNumber(element.width, `${path}.width`) <= 0) {
        fail(`${path}.width must be positive`);
      }
    } else {
      fail(`${path}.kind is invalid`);
    }
  }
  if (decor.length < 48) {
    fail(`${world}.visuals.decor must contain an authored scene composition`);
  }
}

function validatePoints(points, expected, path, bounds) {
  const values = array(points, path);
  if (values.length !== expected) {
    fail(`${path} must contain exactly ${expected} anchors (got ${values.length})`);
  }
  const ids = new Set();
  const coordinates = new Set();
  for (const [index, candidate] of values.entries()) {
    const point = object(candidate, `${path}[${index}]`);
    const id = string(point.id, `${path}[${index}].id`);
    const x = finiteNumber(point.x, `${path}[${index}].x`);
    const y = finiteNumber(point.y, `${path}[${index}].y`);
    if (ids.has(id)) fail(`${path} contains duplicate anchor id ${id}`);
    const coordinate = `${x}:${y}`;
    if (coordinates.has(coordinate)) {
      fail(`${path} contains duplicate coordinate ${coordinate}`);
    }
    if (x < 0 || y < 0 || x > bounds.width || y > bounds.height) {
      fail(`${path}[${index}] is outside map bounds`);
    }
    ids.add(id);
    coordinates.add(coordinate);
  }
}

function validateMap(path, manifestMap, world) {
  const map = object(readJson(path), `${world} map`);
  if (map.type !== "map" || map.orientation !== "orthogonal") {
    fail(`${world} map must be an orthogonal Tiled map`);
  }
  for (const field of ["width", "height"]) {
    if (map[field] !== manifestMap[field]) {
      fail(`${world} map ${field} disagrees with manifest`);
    }
  }
  if (
    map.tilewidth !== manifestMap.tileSize ||
    map.tileheight !== manifestMap.tileSize
  ) {
    fail(`${world} map tile size disagrees with manifest`);
  }
  const layers = array(map.layers, `${world} map.layers`);
  const names = layers.map((layer, index) =>
    string(object(layer, `${world} map.layers[${index}]`).name, "layer.name"),
  );
  for (const required of ["ground", "walk", "collision"]) {
    if (!names.includes(required)) fail(`${world} map is missing ${required} layer`);
  }
}

function validateProvenance(worldsRoot, manifest, runtimePaths, world) {
  const provenancePath = resolve(
    join(worldsRoot, world),
    string(manifest.provenance, `${world}.provenance`),
  );
  if (provenancePath !== join(worldsRoot, "PROVENANCE.json")) {
    fail(`${world} must use the shared PROVENANCE.json`);
  }
  if (!existsSync(provenancePath)) fail(`${world} provenance file does not exist`);
  const provenance = object(readJson(provenancePath), "PROVENANCE.json");
  if (provenance.schemaVersion !== 1) fail("unsupported provenance schema");
  const records = array(provenance.files, "PROVENANCE.json.files");
  if (records.length !== 6) fail("PROVENANCE.json must contain exactly six asset records");
  const byPath = new Map(
    records.map((candidate, index) => {
      const record = object(candidate, `PROVENANCE.json.files[${index}]`);
      return [string(record.path, `files[${index}].path`), record];
    }),
  );
  if (byPath.size !== records.length) fail("PROVENANCE.json contains duplicate paths");
  for (const path of runtimePaths) {
    const rel = relative(worldsRoot, path).replaceAll("\\", "/");
    const record = byPath.get(rel);
    if (!record) fail(`${rel} has no provenance record`);
    for (const field of [
      "author",
      "source",
      "creationDate",
      "generator",
      "promptArtBrief",
      "creationMethod",
      "ownership",
      "license",
      "modificationNote",
      "sha256",
    ]) {
      string(record[field], `${rel} provenance.${field}`);
    }
    if (record.attributionRequired !== false) {
      fail(`${rel} must explicitly declare its attribution requirement`);
    }
    if (!String(record.license).startsWith("LicenseRef-")) {
      fail(`${rel} has an invalid license identifier`);
    }
    if (record.sha256 !== sha256(path)) fail(`${rel} hash mismatch`);
  }
}

function validateWorld(worldsRoot, world) {
  const directory = join(worldsRoot, world);
  const manifestPath = join(directory, "manifest.json");
  const manifest = object(readJson(manifestPath), `${world} manifest`);
  if (manifest.id !== world) fail(`${world} manifest id mismatch`);
  if (manifest.contractVersion !== 1) fail(`${world} contractVersion must be 1`);
  if (!Number.isInteger(manifest.layoutVersion) || manifest.layoutVersion < 1) {
    fail(`${world} layoutVersion must be a positive integer`);
  }

  const map = object(manifest.map, `${world}.map`);
  const tileSize = finiteNumber(map.tileSize, `${world}.map.tileSize`);
  const bounds = {
    width: finiteNumber(map.width, `${world}.map.width`) * tileSize,
    height: finiteNumber(map.height, `${world}.map.height`) * tileSize,
  };
  const declaredLayers = array(map.layers, `${world}.map.layers`).map(
    (layer, index) =>
      string(
        object(layer, `${world}.map.layers[${index}]`).name,
        `${world}.map.layers[${index}].name`,
      ),
  );
  for (const required of ["ground", "walk", "collision"]) {
    if (!declaredLayers.includes(required)) {
      fail(`${world} manifest is missing ${required} layer`);
    }
  }

  const anchors = object(manifest.anchors, `${world}.anchors`);
  for (const [pool, expected] of Object.entries(capacities)) {
    validatePoints(anchors[pool], expected, `${world}.${pool}`, bounds);
  }

  const assets = object(manifest.assets, `${world}.assets`);
  const assetDirectory = join(directory, "assets");
  const assetFiles = readdirSync(assetDirectory).sort();
  const expectedAssetFiles = ["atlas.png", "map.json", "poster.png"];
  if (JSON.stringify(assetFiles) !== JSON.stringify(expectedAssetFiles)) {
    fail(`${world} assets must contain exactly ${expectedAssetFiles.join(", ")}`);
  }
  const mapPath = resolveInside(directory, map.asset, `${world}.map.asset`);
  const atlasPath = resolveInside(
    directory,
    assets.atlas,
    `${world}.assets.atlas`,
  );
  const posterPath = resolveInside(
    directory,
    assets.poster,
    `${world}.assets.poster`,
  );
  validateMap(mapPath, map, world);
  const atlasDimensions = pngDimensions(atlasPath);
  const posterDimensions = pngDimensions(posterPath);
  sameDimensions(atlasDimensions, assets.atlasSize, `${world}.atlasSize`);
  sameDimensions(posterDimensions, assets.posterSize, `${world}.posterSize`);

  const palette = array(manifest.palette, `${world}.palette`);
  const distinctColors = new Set(palette);
  if (
    palette.length < 10 ||
    distinctColors.size < 10 ||
    palette.some((color) => !/^#[0-9a-f]{6}$/i.test(color))
  ) {
    fail(`${world} palette must contain at least ten distinct colors`);
  }
  const visuals = object(manifest.visuals, `${world}.visuals`);
  string(visuals.actorSilhouette, `${world}.visuals.actorSilhouette`);
  string(visuals.stationStyle, `${world}.visuals.stationStyle`);
  const props = array(visuals.props, `${world}.visuals.props`);
  if (props.length < 6) fail(`${world}.visuals.props must define at least six props`);
  props.forEach((prop, index) => string(prop, `${world}.visuals.props[${index}]`));
  const backdropColor = finiteNumber(
    visuals.backdropColor,
    `${world}.visuals.backdropColor`,
  );
  if (
    !Number.isInteger(backdropColor) ||
    backdropColor < 0 ||
    backdropColor >= palette.length
  ) {
    fail(`${world}.visuals.backdropColor must be a palette index`);
  }
  validateDecor(visuals.decor, palette.length, bounds, world);

  const frames = object(assets.frames, `${world}.assets.frames`);
  for (const [name, candidate] of Object.entries(frames)) {
    const frame = object(candidate, `${world}.assets.frames.${name}`);
    const x = finiteNumber(frame.x, `${name}.x`);
    const y = finiteNumber(frame.y, `${name}.y`);
    const width = finiteNumber(frame.width, `${name}.width`);
    const height = finiteNumber(frame.height, `${name}.height`);
    if (
      x < 0 ||
      y < 0 ||
      width <= 0 ||
      height <= 0 ||
      x + width > atlasDimensions.width ||
      y + height > atlasDimensions.height
    ) {
      fail(`${world} frame ${name} exceeds atlas bounds`);
    }
  }

  const clips = object(manifest.clips, `${world}.clips`);
  const clipNames = Object.keys(clips).sort();
  if (JSON.stringify(clipNames) !== JSON.stringify(requiredClips)) {
    fail(`${world} must define exactly the required motion clips`);
  }
  for (const name of requiredClips) {
    const clip = object(clips[name], `${world}.clips.${name}`);
    const variants = array(clip.variants, `${world}.clips.${name}.variants`);
    if (variants.length !== 4) {
      fail(`${world}.${name} must define exactly four variants`);
    }
    for (const [variantIndex, candidate] of variants.entries()) {
      const clipFrames = array(
        candidate,
        `${world}.clips.${name}.variants[${variantIndex}]`,
      );
      if (clipFrames.length < 2) {
        fail(`${world}.${name} variant ${variantIndex} needs at least two frames`);
      }
      for (const frameName of clipFrames) {
        if (!(string(frameName, `${world}.${name}.frame`) in frames)) {
          fail(`${world}.${name} references missing frame ${frameName}`);
        }
      }
    }
    const fps = finiteNumber(clip.fps, `${world}.clips.${name}.fps`);
    if (fps < 8 || fps > 16) fail(`${world}.${name} fps must be 8-16`);
    if (typeof clip.loop !== "boolean") fail(`${world}.${name}.loop must be boolean`);
  }

  const lighting = object(manifest.lighting, `${world}.lighting`);
  if (!lighting.light || !lighting.dark) fail(`${world} needs light and dark lighting`);
  const hitRegions = array(manifest.hitRegions, `${world}.hitRegions`);
  const roles = hitRegions.map((candidate, index) =>
    string(object(candidate, `${world}.hitRegions[${index}]`).role, "hit role"),
  );
  for (const role of ["agent", "squad", "issue"]) {
    if (!roles.includes(role)) fail(`${world} has no ${role} hit region`);
  }

  const runtimePaths = [mapPath, atlasPath, posterPath];
  const transferBytes = runtimePaths.reduce(
    (total, path) => total + statSync(path).size,
    0,
  );
  if (transferBytes > transferBudget) fail(`${world} exceeds 2 MiB pack budget`);
  const decodedBytes =
    (atlasDimensions.width * atlasDimensions.height +
      posterDimensions.width * posterDimensions.height) *
    4;
  if (decodedBytes > decodedBudget) fail(`${world} exceeds 16 MiB decoded budget`);
  validateProvenance(worldsRoot, manifest, runtimePaths, world);

  return {
    manifest,
    assets: runtimePaths.length,
    transferBytes,
    decodedBytes,
  };
}

function assertMaterialDifference(studio, expedition) {
  const checks = [
    studio.map.width !== expedition.map.width,
    studio.visuals.actorSilhouette !== expedition.visuals.actorSilhouette,
    studio.visuals.stationStyle !== expedition.visuals.stationStyle,
    JSON.stringify(studio.visuals.props) !== JSON.stringify(expedition.visuals.props),
    JSON.stringify(studio.palette) !== JSON.stringify(expedition.palette),
    JSON.stringify(studio.anchors.agentStations) !==
      JSON.stringify(expedition.anchors.agentStations),
    JSON.stringify(studio.clips.walk.variants) !==
      JSON.stringify(expedition.clips.walk.variants),
    JSON.stringify(studio.visuals.decor) !==
      JSON.stringify(expedition.visuals.decor),
  ];
  if (checks.some((different) => !different)) {
    fail("Studio and Expedition must differ in geometry, actors, props, stations, palette, and motion");
  }
}

function assertPosterPixelDifference(worldsRoot) {
  const studio = pngPixels(join(worldsRoot, "studio", "assets", "poster.png"));
  const expedition = pngPixels(
    join(worldsRoot, "expedition", "assets", "poster.png"),
  );
  const ratio = pixelDifferenceRatio(studio, expedition);
  if (ratio < 0.35) {
    fail(
      `Studio and Expedition poster pixels must be materially different (ratio ${ratio.toFixed(3)})`,
    );
  }
}

export function validateOfficeAssets(root = process.cwd()) {
  const worldsRoot = join(root, "packages/views/office/worlds");
  const worlds = readdirSync(worldsRoot, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort();
  if (JSON.stringify(worlds) !== JSON.stringify(expectedWorlds)) {
    fail(`expected exactly two worlds: ${expectedWorlds.join(", ")}`);
  }
  const attribution = join(worldsRoot, "ATTRIBUTION.md");
  if (!statSync(attribution, { throwIfNoEntry: false })?.isFile()) {
    fail("ATTRIBUTION.md is missing");
  }
  const results = Object.fromEntries(
    worlds.map((world) => [world, validateWorld(worldsRoot, world)]),
  );
  assertMaterialDifference(results.studio.manifest, results.expedition.manifest);
  assertPosterPixelDifference(worldsRoot);
  return {
    worlds,
    assets: Object.values(results).reduce((sum, result) => sum + result.assets, 0),
    transferBytes: Object.fromEntries(
      worlds.map((world) => [world, results[world].transferBytes]),
    ),
    decodedBytes: Object.fromEntries(
      worlds.map((world) => [world, results[world].decodedBytes]),
    ),
  };
}

const invokedPath = process.argv[1] ? resolve(process.argv[1]) : "";
if (invokedPath === fileURLToPath(import.meta.url)) {
  try {
    const result = validateOfficeAssets();
    process.stdout.write(
      `Office assets valid (${result.worlds.length} worlds, ${result.assets} runtime assets).\n`,
    );
  } catch (error) {
    process.stderr.write(`${error instanceof Error ? error.message : "Office asset validation failed"}\n`);
    process.exitCode = 1;
  }
}
