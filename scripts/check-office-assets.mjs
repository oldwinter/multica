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
    const clipFrames = array(clip.frames, `${world}.clips.${name}.frames`);
    if (clipFrames.length < 2) fail(`${world}.${name} needs at least two frames`);
    for (const frameName of clipFrames) {
      if (!(string(frameName, `${world}.${name}.frame`) in frames)) {
        fail(`${world}.${name} references missing frame ${frameName}`);
      }
    }
    const fps = finiteNumber(clip.fps, `${world}.clips.${name}.fps`);
    if (fps < 8 || fps > 16) fail(`${world}.${name} fps must be 8-16`);
    if (typeof clip.loop !== "boolean") fail(`${world}.${name}.loop must be boolean`);
  }

  const palette = array(manifest.palette, `${world}.palette`);
  if (palette.length < 6 || palette.some((color) => !/^#[0-9a-f]{6}$/i.test(color))) {
    fail(`${world} palette must contain at least six hex colors`);
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
    JSON.stringify(studio.clips.walk.frames) !==
      JSON.stringify(expedition.clips.walk.frames),
  ];
  if (checks.some((different) => !different)) {
    fail("Studio and Expedition must differ in geometry, actors, props, stations, palette, and motion");
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
