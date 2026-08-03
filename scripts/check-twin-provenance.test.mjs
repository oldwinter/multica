import assert from "node:assert/strict";
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const repoRoot = fileURLToPath(new URL("..", import.meta.url));
const matrixPath = join(repoRoot, "docs/downstream/twin/legacy-migration-matrix.md");
const validatorPath = join(repoRoot, "scripts/check-twin-provenance.mjs");
const legacyRoot = process.env.TWIN_LEGACY_ROOT;

function runValidator(matrix = matrixPath) {
  const args = [validatorPath, "--matrix", matrix];
  if (legacyRoot) args.push("--legacy-root", legacyRoot);
  return spawnSync(process.execPath, args, {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function withMatrixMutation(mutate, assertion) {
  const directory = mkdtempSync(join(tmpdir(), "multica-twin-provenance-"));
  const copy = join(directory, "legacy-migration-matrix.md");
  cpSync(matrixPath, copy);

  try {
    mutate(copy);
    assertion(runValidator(copy));
  } finally {
    rmSync(directory, { force: true, recursive: true });
  }
}

function mutateMatrix(path, mutate) {
  const source = readFileSync(path, "utf8");
  const match = source.match(/```json\n([\s\S]*?)\n```/);
  assert.ok(match, "matrix fixture must contain one JSON data block");
  const matrix = JSON.parse(match[1]);
  mutate(matrix);
  writeFileSync(path, source.replace(match[0], `\`\`\`json\n${JSON.stringify(matrix, null, 2)}\n\`\`\``));
}

const provenanceTest = (name, fn) => test(name, { skip: !legacyRoot }, fn);

provenanceTest("Given the checked-in matrix, when all provenance is present, then validation succeeds", () => {
  const result = runValidator();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /provenance valid/);
});

provenanceTest("Given a mapping to a missing legacy source, when validation runs, then it names the source", () => {
  withMatrixMutation(
    (path) => mutateMatrix(path, (matrix) => {
      matrix.mappings[0].sourcePath = "missing-source.md";
    }),
    (result) => {
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /missing legacy source: missing-source\.md/);
    },
  );
});

provenanceTest("Given a stale legacy commit, when validation runs, then it names the stale commit", () => {
  withMatrixMutation(
    (path) => mutateMatrix(path, (matrix) => {
      matrix.legacySourceCommit = "0000000000000000000000000000000000000000";
    }),
    (result) => {
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /stale legacy source commit/);
    },
  );
});

provenanceTest("Given duplicate concept mappings, when validation runs, then it rejects the concept", () => {
  withMatrixMutation(
    (path) => mutateMatrix(path, (matrix) => {
      matrix.mappings.push({ ...matrix.mappings[0], id: "term-twin-duplicate" });
    }),
    (result) => {
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /duplicate concept: Twin/);
    },
  );
});

provenanceTest("Given an adopted story without a mapping, when validation runs, then it names the story", () => {
  withMatrixMutation(
    (path) => mutateMatrix(path, (matrix) => {
      matrix.mappings = matrix.mappings.filter((mapping) => mapping.id !== "story-45");
    }),
    (result) => {
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /missing adopted story mapping: 45/);
    },
  );
});

provenanceTest("Given a destination outside the downstream namespace, when validation runs, then it rejects it", () => {
  withMatrixMutation(
    (path) => mutateMatrix(path, (matrix) => {
      matrix.mappings[0].destination = "docs/product-overview.md";
    }),
    (result) => {
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /destination outside downstream namespace: docs\/product-overview\.md/);
    },
  );
});
