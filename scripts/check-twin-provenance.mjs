import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { spawnSync } from "node:child_process";

const LEGACY_COMMIT = "1c864890c496115a65e6eafc3e4193caa157aee1";
const MULTICA_COMMIT = "37f3bb7dd9c0fe665051ce26dadab03b090dc1af";
const DOWNSTREAM_NAMESPACE = "docs/downstream/twin/";
const LEGACY_PATHS = new Set([
  "CONTEXT.md",
  ".scratch/mvp-spec/spec.md",
  "DESIGN.md",
  "docs/adr/0001-self-build-selective-borrowing.md",
  "docs/adr/0002-typescript-local-web-sqlite.md",
  "docs/adr/0003-twin-injection-protocol.md",
]);
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

class ProvenanceError extends Error {}

function fail(message) {
  throw new ProvenanceError(message);
}

function readArgument(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  const value = process.argv[index + 1];
  if (value === undefined || value.startsWith("--")) fail(`missing value for ${name}`);
  return value;
}

function rejectUnknownArguments() {
  const permitted = new Set(["--matrix", "--legacy-root"]);
  for (const argument of process.argv.slice(2)) {
    if (argument.startsWith("--") && !permitted.has(argument)) fail(`unknown argument: ${argument}`);
  }
}

function readMatrix(path) {
  const source = readFileSync(path, "utf8");
  const blocks = [...source.matchAll(/```json\n([\s\S]*?)\n```/g)];
  if (blocks.length !== 1) fail("matrix must contain exactly one JSON data block");

  try {
    return JSON.parse(blocks[0][1]);
  } catch (error) {
    if (error instanceof SyntaxError) fail(`invalid matrix JSON: ${error.message}`);
    throw error;
  }
}

function requireObject(value, label) {
  if (value === null || typeof value !== "object" || Array.isArray(value)) fail(`invalid ${label}`);
  return value;
}

function requireString(value, label) {
  if (typeof value !== "string" || value.length === 0) fail(`invalid ${label}`);
  return value;
}

function requireNumber(value, label) {
  if (!Number.isInteger(value) || value < 1) fail(`invalid ${label}`);
  return value;
}

function requireArray(value, label) {
  if (!Array.isArray(value)) fail(`invalid ${label}`);
  return value;
}

function legacyFile(sourcePath) {
  return sourcePath.split("#", 1)[0];
}

function git(legacyRoot, args, failureMessage) {
  const result = spawnSync("git", ["-C", legacyRoot, ...args], { encoding: "utf8" });
  if (result.status !== 0) fail(`${failureMessage}: ${result.stderr.trim()}`);
  return result.stdout.trim();
}

function validateSource(legacyRoot, sourcePath, sources) {
  const file = legacyFile(sourcePath);
  if (!LEGACY_PATHS.has(file)) fail(`missing legacy source: ${file}`);
  if (sources.has(file)) return;
  git(legacyRoot, ["cat-file", "-e", `${LEGACY_COMMIT}:${file}`], `missing legacy source: ${file}`);
  sources.add(file);
}

function validateDestination(destination) {
  if (destination === "Scope OUT") return;
  if (!destination.startsWith(DOWNSTREAM_NAMESPACE)) {
    fail(`destination outside downstream namespace: ${destination}`);
  }
  const document = legacyFile(destination);
  if (!existsSync(resolve(repoRoot, document))) fail(`missing downstream destination: ${document}`);
}

function validateMapping(legacyRoot, mapping, ids, concepts, stories, sources) {
  const item = requireObject(mapping, "mapping");
  const id = requireString(item.id, "mapping id");
  if (ids.has(id)) fail(`duplicate mapping id: ${id}`);
  ids.add(id);

  const kind = requireString(item.kind, `mapping kind for ${id}`);
  const adoption = requireString(item.adoption, `mapping adoption for ${id}`);
  if (!["adopted", "adapted", "out-of-scope"].includes(adoption)) fail(`invalid adoption for ${id}`);
  validateSource(legacyRoot, requireString(item.sourcePath, `source path for ${id}`), sources);
  validateDestination(requireString(item.destination, `destination for ${id}`));

  if (kind === "concept") {
    const concept = requireString(item.concept, `concept for ${id}`);
    if (concepts.has(concept)) fail(`duplicate concept: ${concept}`);
    concepts.set(concept, adoption);
    return;
  }
  if (kind === "story") {
    const story = requireNumber(item.story, `story for ${id}`);
    if (stories.has(story)) fail(`duplicate story: ${story}`);
    stories.set(story, adoption);
    return;
  }
  fail(`invalid mapping kind for ${id}`);
}

function validateRequired(required, observed, label) {
  for (const value of requireArray(required, `required ${label}s`)) {
    const key = label === "story" ? requireNumber(value, `required ${label}`) : requireString(value, `required ${label}`);
    const adoption = observed.get(key);
    if (adoption === undefined || adoption === "out-of-scope") fail(`missing adopted ${label} mapping: ${key}`);
  }
}

function validate(matrix, legacyRoot) {
  const data = requireObject(matrix, "matrix");
  if (data.schemaVersion !== 1) fail("unsupported matrix schema version");
  if (data.legacySourceCommit !== LEGACY_COMMIT) fail("stale legacy source commit");
  if (data.multicaSourceCommit !== MULTICA_COMMIT) fail("stale Multica source commit");
  git(legacyRoot, ["cat-file", "-e", `${LEGACY_COMMIT}^{commit}`], "legacy source commit is unavailable");

  const concepts = new Map();
  const stories = new Map();
  const ids = new Set();
  const sources = new Set();
  for (const mapping of requireArray(data.mappings, "mappings")) {
    validateMapping(legacyRoot, mapping, ids, concepts, stories, sources);
  }
  validateRequired(data.requiredAdoptedConcepts, concepts, "concept");
  validateRequired(data.requiredAdoptedStories, stories, "story");
}

function main() {
  rejectUnknownArguments();
  const matrix = readArgument("--matrix", resolve(repoRoot, "docs/downstream/twin/legacy-migration-matrix.md"));
  const legacyRoot = readArgument("--legacy-root", process.env.TWIN_LEGACY_ROOT);
  if (!legacyRoot) fail("legacy root required: pass --legacy-root or set TWIN_LEGACY_ROOT");
  validate(readMatrix(matrix), legacyRoot);
  process.stdout.write("twin provenance valid\n");
}

try {
  main();
} catch (error) {
  if (error instanceof Error) process.stderr.write(`${error.message}\n`);
  else process.stderr.write("unknown validator failure\n");
  process.exitCode = 1;
}
