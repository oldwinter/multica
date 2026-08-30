import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import {
  copyFileSync,
  cpSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
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

test("the validator rejects a one-note world palette", () => {
  assert.throws(
    () =>
      withWorldFixture((root) => {
        const path = join(root, worldsRoot, "studio", "manifest.json");
        const manifest = JSON.parse(readFileSync(path, "utf8"));
        manifest.palette = Array.from({ length: 10 }, () => "#20262d");
        writeFileSync(path, `${JSON.stringify(manifest, null, 2)}\n`);
      }),
    /palette must contain at least ten distinct colors/,
  );
});

test("the validator rejects malformed shared scene decoration", () => {
  assert.throws(
    () =>
      withWorldFixture((root) => {
        const path = join(root, worldsRoot, "expedition", "manifest.json");
        const manifest = JSON.parse(readFileSync(path, "utf8"));
        manifest.visuals.decor = [
          { kind: "rect", x: 0, y: 0, width: 32, height: 32, color: 99 },
        ];
        writeFileSync(path, `${JSON.stringify(manifest, null, 2)}\n`);
      }),
    /visuals\.decor.*palette index/,
  );
});

test("the validator requires four authored variants for every motion clip", () => {
  assert.throws(
    () =>
      withWorldFixture((root) => {
        const path = join(root, worldsRoot, "studio", "manifest.json");
        const manifest = JSON.parse(readFileSync(path, "utf8"));
        manifest.clips.idle.variants = [];
        writeFileSync(path, `${JSON.stringify(manifest, null, 2)}\n`);
      }),
    /idle must define exactly four variants/,
  );
});

test("the validator rejects visually identical world posters", () => {
  assert.throws(
    () =>
      withWorldFixture((root) => {
        const studioPoster = join(
          root,
          worldsRoot,
          "studio",
          "assets",
          "poster.png",
        );
        const expeditionPoster = join(
          root,
          worldsRoot,
          "expedition",
          "assets",
          "poster.png",
        );
        copyFileSync(studioPoster, expeditionPoster);
        const provenancePath = join(root, worldsRoot, "PROVENANCE.json");
        const provenance = JSON.parse(readFileSync(provenancePath, "utf8"));
        const record = provenance.files.find(
          (candidate) =>
            candidate.path === "expedition/assets/poster.png",
        );
        record.sha256 = createHash("sha256")
          .update(readFileSync(expeditionPoster))
          .digest("hex");
        writeFileSync(
          provenancePath,
          `${JSON.stringify(provenance, null, 2)}\n`,
        );
      }),
    /poster pixels must be materially different/,
  );
});
