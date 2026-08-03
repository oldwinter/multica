import assert from "node:assert/strict";
import { cpSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

const requiredContracts = [
  "docs/downstream/twin/adr/0001-selective-borrowing.md",
  "docs/downstream/twin/adr/0002-domain-and-storage-boundaries.md",
  "docs/downstream/twin/adr/0003-daemon-profile-and-auth.md",
  "docs/downstream/twin/adr/0004-execution-egress-and-recovery.md",
  "docs/downstream/twin/contracts/authorization.md",
  "docs/downstream/twin/contracts/field-placement.md",
  "docs/downstream/twin/contracts/lifecycle.md",
  "docs/downstream/twin/contracts/task-isolation.md",
  "docs/downstream/twin/contracts/runtime-topology.md",
  "docs/downstream/twin/contracts/credential-and-egress.md",
  "docs/downstream/twin/contracts/migration-and-recovery.md",
];

test("the complete Twin contract set exists", () => {
  const missing = requiredContracts.filter((path) => !existsSync(path));
  assert.deepEqual(missing, [], `missing Twin contracts: ${missing.join(", ")}`);
});

function runChecker(contractsRoot = process.cwd()) {
  return spawnSync(process.execPath, [
    "scripts/check-twin-contracts.mjs",
    "--contracts-root", contractsRoot,
    "--repo-root", process.cwd(),
  ], { cwd: process.cwd(), encoding: "utf8" });
}

function withContracts(mutator) {
  const root = mkdtempSync(join(tmpdir(), "twin-contract-test-"));
  cpSync("docs", join(root, "docs"), { recursive: true });
  try {
    mutator(root);
    return runChecker(root);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

test("the repository contracts validate", () => {
  const result = runChecker();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /^Twin contracts valid \(\d+ tables, \d+ rows, \d+ direct consumers\)\.\n$/);
});

test("machine identity cannot replace immutable owner/profile authorization", () => {
  const result = withContracts((root) => {
    const path = join(root, "docs/downstream/twin/adr/0003-daemon-profile-and-auth.md");
    const input = readFileSync(path, "utf8");
    writeFileSync(path, input.replace("workspace+machine+owner+profile and current callback generation", "workspace+machine and current callback generation"));
  });
  assert.notEqual(result.status, 0);
  assert.equal(result.stdout, "");
  assert.match(result.stderr, /contract semantics changed: auth-callback-identity/);
});

test("exchange authorization is reloaded after transaction locks", () => {
  const result = withContracts((root) => {
    const path = join(root, "docs/downstream/twin/contracts/credential-and-egress.md");
    const input = readFileSync(path, "utf8");
    writeFileSync(path, input.replace("PAT+membership+workspace+machine+owner+profile+generation", "PAT+workspace+machine+owner+profile+generation"));
  });
  assert.notEqual(result.status, 0);
  assert.equal(result.stdout, "");
  assert.match(result.stderr, /contract semantics changed: exchange-postlock-auth/);
});

test("every direct task consumer has one explicit classification", () => {
  const result = withContracts((root) => {
    const path = join(root, "docs/downstream/twin/contracts/task-isolation.md");
    const input = readFileSync(path, "utf8");
    writeFileSync(path, input.split("\n").filter((line) => !line.includes("consumer:server/pkg/db/queries/chat.sql")).join("\n"));
  });
  assert.notEqual(result.status, 0);
  assert.equal(result.stdout, "");
  assert.match(result.stderr, /contract semantic set changed/);
});

test("workspace deletion remains owner-only when the row ID is unchanged", () => {
  const result = withContracts((root) => {
    const path = join(root, "docs/downstream/twin/contracts/authorization.md");
    const input = readFileSync(path, "utf8");
    writeFileSync(path, input.replace("authenticated workspace owner only", "workspace admin"));
  });
  assert.notEqual(result.status, 0, "semantic corruption must fail validation");
  assert.equal(result.stdout, "");
  assert.match(result.stderr, /contract semantics changed: action-workspace-delete/);
});

test("a new named query in a known SQL file requires classification", () => {
  const root = mkdtempSync(join(tmpdir(), "twin-contract-repo-test-"));
  cpSync("server", join(root, "server"), { recursive: true });
  try {
    const path = join(root, "server/pkg/db/queries/chat.sql");
    const input = readFileSync(path, "utf8");
    writeFileSync(path, `${input}\n-- name: UnclassifiedLinkedBlindSpot :many\nSELECT * FROM agent_task_queue;\n`);
    const result = spawnSync(process.execPath, [
      "scripts/check-twin-contracts.mjs",
      "--contracts-root", process.cwd(),
      "--repo-root", root,
    ], { cwd: process.cwd(), encoding: "utf8" });
    assert.notEqual(result.status, 0, "new direct SQL consumer must fail validation");
    assert.equal(result.stdout, "");
    assert.match(result.stderr, /unclassified agent_task_queue SQL consumer: server\/pkg\/db\/queries\/chat.sql#UnclassifiedLinkedBlindSpot/);
    writeFileSync(path, input);
    const goPath = join(root, "server/internal/metrics/business_sampler_queries.go");
    const goInput = readFileSync(goPath, "utf8");
    writeFileSync(goPath, `${goInput}\nfunc unclassifiedLinkedBlindSpot() { _ = \`SELECT * FROM agent_task_queue\` }\n`);
    const goResult = spawnSync(process.execPath, [
      "scripts/check-twin-contracts.mjs",
      "--contracts-root", process.cwd(),
      "--repo-root", root,
    ], { cwd: process.cwd(), encoding: "utf8" });
    assert.notEqual(goResult.status, 0, "new direct Go consumer must fail validation");
    assert.equal(goResult.stdout, "");
    assert.match(goResult.stderr, /unclassified agent_task_queue Go consumer: server\/internal\/metrics\/business_sampler_queries.go#unclassifiedLinkedBlindSpot/);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("Go consumer discovery preserves strings and classifies package, generic, and receiver identities", () => {
  const root = mkdtempSync(join(tmpdir(), "twin-contract-go-scan-test-"));
  cpSync("server", join(root, "server"), { recursive: true });
  const fixturePath = join(root, "server/internal/metrics/twin_contract_fixture.go");
  const runFixture = (source, expectedIdentity) => {
    writeFileSync(fixturePath, source);
    const result = spawnSync(process.execPath, [
      "scripts/check-twin-contracts.mjs",
      "--contracts-root", process.cwd(),
      "--repo-root", root,
    ], { cwd: process.cwd(), encoding: "utf8" });
    assert.notEqual(result.status, 0, "unclassified Go consumer must fail validation");
    assert.equal(result.stdout, "");
    assert.match(result.stderr, new RegExp(`unclassified agent_task_queue Go consumer: server/internal/metrics/twin_contract_fixture\\.go#${expectedIdentity}`));
  };

  try {
    runFixture(`package metrics\n\n// SELECT * FROM agent_task_queue\nconst callbackURL = "https://example.invalid/agent_task_queue"\nconst packageQuery = \`SELECT * FROM agent_task_queue\`\n`, "package");
    runFixture(`package metrics\n\nfunc genericLinked[T any]() { _ = \`SELECT * FROM agent_task_queue\` }\n`, "genericLinked");
    runFixture(`package metrics\n\ntype fixtureReceiver struct{}\nfunc (fixtureReceiver) duplicate() { _ = \`SELECT * FROM agent_task_queue\` }\n`, "fixtureReceiver\\.duplicate");
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("all 225 structured rows reject semantic corruption or deletion", { timeout: 120_000 }, () => {
  const root = mkdtempSync(join(tmpdir(), "twin-contract-matrix-test-"));
  cpSync("docs", join(root, "docs"), { recursive: true });
  try {
    const rows = [];
    for (const path of requiredContracts) {
      const source = readFileSync(path, "utf8");
      source.split("\n").forEach((line) => {
        if (/^\| [a-z0-9][^|]+ \|/.test(line) && !line.startsWith("| id |")) rows.push({ path, line });
      });
    }
    assert.equal(rows.length, 225);
    rows.forEach(({ path, line }, index) => {
      const fixturePath = join(root, path);
      const original = readFileSync(fixturePath, "utf8");
      const id = line.split("|")[1].trim();
      const cells = line.split("|");
      cells[2] = ` ${cells[2].trim()}-corrupt `;
      const replacement = index % 2 === 0 ? cells.join("|") : "";
      writeFileSync(fixturePath, original.replace(line, replacement));
      const result = runChecker(root);
      assert.notEqual(result.status, 0, `row accepted after corruption: ${id}`);
      assert.equal(result.stdout, "", `row emitted success after corruption: ${id}`);
      if (index % 2 === 0) {
        assert.match(result.stderr, /contract semantics changed:/, `corrupt row lacked semantic diagnostic: ${id}`);
      } else {
        assert.match(result.stderr, /missing invariant:|contract semantic set changed/, `deleted row lacked set diagnostic: ${id}`);
      }
      writeFileSync(fixturePath, original);
    });
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
