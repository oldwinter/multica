import assert from "node:assert/strict";
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { validateOfficeAssets } from "./check-office-assets.mjs";

const worldsRoot = "packages/views/office/worlds";

function withWorldFixture(mutator) {
  const root = mkdtempSync(join(tmpdir(), "office-assets-test-"));
  cpSync(worldsRoot, join(root, worldsRoot), { recursive: true });
  try {
    mutator?.(root);
    return validateOfficeAssets(root);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

test("the checked-in Office assets satisfy the complete contract", () => {
  const result = validateOfficeAssets(process.cwd());
  assert.deepEqual(result.worlds, ["expedition", "studio"]);
  assert.equal(result.assets, 6);
});

test("the validator rejects a provenance hash mismatch", () => {
  assert.throws(
    () =>
      withWorldFixture((root) => {
        const path = join(root, worldsRoot, "PROVENANCE.json");
        const provenance = JSON.parse(readFileSync(path, "utf8"));
        provenance.files[0].sha256 = "0".repeat(64);
        writeFileSync(path, `${JSON.stringify(provenance, null, 2)}\n`);
      }),
    /hash mismatch/,
  );
});

test("the validator rejects a pack below the scene capacity", () => {
  assert.throws(
    () =>
      withWorldFixture((root) => {
        const path = join(root, worldsRoot, "studio", "manifest.json");
        const manifest = JSON.parse(readFileSync(path, "utf8"));
        manifest.anchors.agentStations.pop();
        writeFileSync(path, `${JSON.stringify(manifest, null, 2)}\n`);
      }),
    /agentStations must contain exactly 40 anchors/,
  );
});
