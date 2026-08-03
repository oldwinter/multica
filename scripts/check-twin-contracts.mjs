#!/usr/bin/env node

import { readFileSync, readdirSync, statSync } from "node:fs";
import { createHash } from "node:crypto";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const contractPaths = [
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

const required = {
  decisions: ["decision-selective-borrowing", "decision-existing-domain", "decision-scope", "decision-claude-only", "decision-observations", "decision-evidence-custody"],
  "domain-boundaries": ["boundary-twin", "boundary-topic", "boundary-run", "boundary-input", "boundary-profile"],
  "auth-decisions": ["auth-callback-identity", "auth-token-kind", "auth-machine-insufficient", "auth-local-control", "auth-diagnostics-exception"],
  "execution-decisions": ["execution-workspace", "execution-possession", "execution-claude", "execution-effect", "execution-ownership"],
  actions: ["action-create", "action-central-lifecycle", "action-local-content", "action-daemon-append", "action-cycle-control", "action-assume-cycle", "action-clear-run", "action-hard-stop", "action-budget-approve", "action-budget-decline", "action-run-accept", "action-workspace-delete", "action-post-delete"],
  "review-attestations": ["attest-egress", "attest-uncertain", "attest-sign", "attest-deposition", "attest-run-review"],
  fields: ["field-shared-metadata", "field-local-resource", "field-local-artifact", "field-local-secret", "field-profile-opaque", "field-safe-progress", "field-topic-body", "field-report", "field-deposition", "field-label", "field-global-search"],
  "source-policy": ["policy-local-only", "policy-remote-allowed", "policy-unknown", "policy-tighten", "policy-loosen"],
  "lifecycle-controls": ["lifecycle-logout", "lifecycle-retire", "lifecycle-abandon", "lifecycle-member-revoke", "lifecycle-profile-revoke", "lifecycle-workspace-delete", "lifecycle-reinvite"],
  "cleanup-leases": ["cleanup-issue", "cleanup-claim", "cleanup-ack", "cleanup-abandon", "cleanup-revoke"],
  retention: ["retention-signed", "retention-run", "retention-staging", "retention-audit", "retention-local-source"],
  "daemon-task-routes": ["daemon-claim", "daemon-prepare", "daemon-status", "daemon-wait-local", "daemon-message", "daemon-cancel-gc", "daemon-recovery", "daemon-terminal-locality"],
  "ordinary-task-behavior": ["ordinary-claim", "ordinary-session", "ordinary-comments", "ordinary-mutations", "ordinary-projections", "ordinary-aggregates", "ordinary-storage", "linked-storage", "rerun-source"],
  indexes: ["index-ordinary-pending", "index-linked-operation", "index-file-count", "index-enqueue-proof"],
  topology: ["topology-creator", "topology-register", "topology-register-writes", "topology-enqueue", "topology-bind", "topology-agent-move", "topology-agent-qtx", "topology-builder", "topology-profile", "topology-gc", "topology-teardown", "topology-issue-delete"],
  realtime: ["realtime-http", "realtime-ws", "realtime-wakeup", "realtime-ordinary", "realtime-sequence"],
  exchange: ["exchange-selector", "exchange-preflight", "exchange-postlock-auth", "exchange-serialize", "exchange-attempt", "exchange-replay", "exchange-conflict", "exchange-recovery", "exchange-client"],
  "credential-store": ["credential-module", "credential-build", "credential-key", "credential-helper", "credential-scenario", "credential-headless", "credential-macos", "credential-linux", "credential-workflow"],
  "model-budget-effects": ["brain-profile", "brain-budget", "run-budget", "budget-extension", "ask-time", "model-gateway", "effect-adapter", "run-input", "run-report", "aggregate-report", "deposition"],
  migrations: ["migration-table-identity", "migration-pending-files", "migration-receipt", "migration-barrier"],
  "recovery-operations": ["recovery-artifact", "recovery-token", "recovery-import", "recovery-generation", "recovery-sign", "recovery-run", "recovery-effect", "recovery-deposition"],
  "platform-proof": ["platform-macos", "platform-linux", "platform-executable", "platform-receipt-ownership"],
  "refinement-rejections": Array.from({ length: 29 }, (_, index) => `round-${index + 14}`),
  "final-custody": ["custody-plan", "custody-runner", "custody-platform", "custody-qa", "custody-upstream", "custody-tools"],
};

const semanticRoot = "602cd6dcd9ecbea34a8054c8cf3287570c49b66a9870d09c11ed8962b72cbde7";

function parseArgs(argv) {
  const here = resolve(dirname(fileURLToPath(import.meta.url)), "..");
  const args = { contractsRoot: here, repoRoot: here };
  for (let i = 0; i < argv.length; i += 2) {
    const key = argv[i];
    const value = argv[i + 1];
    if (!value || !["--contracts-root", "--repo-root"].includes(key)) throw new Error(`invalid argument: ${key ?? "<missing>"}`);
    args[key === "--contracts-root" ? "contractsRoot" : "repoRoot"] = resolve(value);
  }
  return args;
}

function parseTables(text, source) {
  const lines = text.split(/\r?\n/);
  const tables = [];
  for (let i = 0; i < lines.length; i += 1) {
    const match = lines[i].match(/^<!-- twin-contract: ([a-z0-9-]+) -->$/);
    if (!match) continue;
    const name = match[1];
    const header = lines[++i];
    const divider = lines[++i];
    if (!header?.startsWith("|") || !/^\|(?:\s*:?-+:?\s*\|)+$/.test(divider ?? "")) throw new Error(`${source}: malformed table ${name}`);
    const columns = header.split("|").slice(1, -1).map((cell) => cell.trim());
    if (columns.at(-1) !== "semantic_sha256") throw new Error(`${source}: unsealed table ${name}`);
    const rows = [];
    while (lines[i + 1]?.startsWith("|")) {
      const cells = lines[++i].split("|").slice(1, -1).map((cell) => cell.trim());
      if (cells.length !== columns.length || cells.some((cell) => !cell)) throw new Error(`${source}: malformed row in ${name}`);
      const semantic = cells.pop();
      const actual = createHash("sha256").update([name, ...cells].join("\0")).digest("hex");
      if (semantic !== actual) throw new Error(`contract semantics changed: ${cells[0]}`);
      rows.push({ id: cells[0], cells, semantic, text: cells.join(" | ") });
    }
    tables.push({ name, rows, source });
  }
  return tables;
}

function scanGoSource(source) {
  const code = [];
  const literals = [];
  let state = "code";
  let quote = "";
  let literal = "";

  for (let index = 0; index < source.length;) {
    const current = source[index];
    const next = source[index + 1];

    if (state === "line-comment") {
      if (current === "\n") {
        code.push(current);
        state = "code";
      } else {
        code.push(" ");
      }
      index += 1;
      continue;
    }

    if (state === "block-comment") {
      if (current === "*" && next === "/") {
        code.push(" ", " ");
        index += 2;
        state = "code";
      } else {
        code.push(current === "\n" ? "\n" : " ");
        index += 1;
      }
      continue;
    }

    if (state === "literal") {
      if (quote !== "`" && current === "\\") {
        code.push(current);
        literal += current;
        if (next !== undefined) {
          code.push(next);
          literal += next;
          index += 2;
        } else {
          index += 1;
        }
        continue;
      }
      if (current === quote) {
        code.push(current);
        literals.push(literal);
        literal = "";
        quote = "";
        state = "code";
      } else {
        code.push(current);
        literal += current;
      }
      index += 1;
      continue;
    }

    if (current === "/" && next === "/") {
      code.push(" ", " ");
      index += 2;
      state = "line-comment";
      continue;
    }
    if (current === "/" && next === "*") {
      code.push(" ", " ");
      index += 2;
      state = "block-comment";
      continue;
    }
    if (["\"", "'", "`"].includes(current)) {
      code.push(current);
      literal = "";
      quote = current;
      state = "literal";
      index += 1;
      continue;
    }
    code.push(current);
    index += 1;
  }

  return { code: code.join(""), literals: literals.join("\n") };
}

function directGoConsumer(body) {
  const { code, literals } = scanGoSource(body);
  return /\bagent_task_queue\b/.test(literals) && (
    /\b(?:SELECT|INSERT|UPDATE|DELETE)\b/.test(literals) || /\.(?:Query|QueryRow|Exec)\s*\(/.test(code)
  );
}

function receiverType(receiver) {
  if (!receiver) return "";
  const value = receiver.slice(1, -1).trim().replace(/^[_A-Za-z]\w*\s+/, "");
  return value.replace(/^\*/, "").replace(/\[[^\]]*\]$/, "").trim();
}

function goFunctions(source) {
  return [...source.matchAll(/^func\s+(?:(\([^)]*\))\s*)?([A-Za-z_]\w*)\s*(?:\[[^\]]*\]\s*)?\(/gm)].map((match) => ({
    index: match.index,
    identity: `${receiverType(match[1]) ? `${receiverType(match[1])}.` : ""}${match[2]}`,
  }));
}

function walk(dir, root, found) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    const rel = relative(root, path).replaceAll("\\", "/");
    if (entry.isDirectory()) {
      if (["server/pkg/db/generated", "server/migrations"].some((prefix) => rel === prefix || rel.startsWith(`${prefix}/`))) continue;
      walk(path, root, found);
    } else if (entry.name.endsWith(".sql")) {
      const text = readFileSync(path, "utf8");
      const queries = [...text.matchAll(/^-- name: (\S+)\s+:[^\n]*$/gm)];
      for (let i = 0; i < queries.length; i += 1) {
        const body = text.slice(queries[i].index + queries[i][0].length, queries[i + 1]?.index ?? text.length).replace(/^--.*$/gm, "");
        if (/\bagent_task_queue\b/.test(body)) found.add(`${rel}#query:${queries[i][1]}`);
      }
    } else if (entry.name.endsWith(".go") && !entry.name.endsWith("_test.go")) {
      const text = readFileSync(path, "utf8");
      const functions = goFunctions(text);
      if (directGoConsumer(text.slice(0, functions[0]?.index ?? text.length))) found.add(`${rel}#go:package`);
      for (let i = 0; i < functions.length; i += 1) {
        const body = text.slice(functions[i].index, functions[i + 1]?.index ?? text.length);
        if (directGoConsumer(body)) found.add(`${rel}#go:${functions[i].identity}`);
      }
    }
  }
}

function validate() {
  const { contractsRoot, repoRoot } = parseArgs(process.argv.slice(2));
  const tables = [];
  for (const rel of contractPaths) {
    const path = join(contractsRoot, rel);
    if (!statSync(path, { throwIfNoEntry: false })?.isFile()) throw new Error(`missing contract document: ${rel}`);
    tables.push(...parseTables(readFileSync(path, "utf8"), rel));
  }
  const byName = new Map();
  const ids = new Map();
  for (const table of tables) {
    if (byName.has(table.name)) throw new Error(`duplicate contract table: ${table.name}`);
    byName.set(table.name, table);
    for (const row of table.rows) {
      if (ids.has(row.id)) throw new Error(`overlapping contract row: ${row.id}`);
      ids.set(row.id, row);
    }
  }
  for (const [name, expected] of Object.entries(required)) {
    const table = byName.get(name);
    if (!table) throw new Error(`missing contract table: ${name}`);
    for (const id of expected) if (!table.rows.some((row) => row.id === id)) throw new Error(`missing invariant: ${id}`);
  }
  const root = createHash("sha256").update(tables.flatMap((table) => table.rows.map((row) => `${row.id}\0${row.semantic}`)).join("\n")).digest("hex");
  if (root !== semanticRoot) throw new Error("contract semantic set changed");
  const callback = ids.get("auth-callback-identity").text;
  if (!callback.includes("workspace+machine+owner+profile") || !callback.includes("generation")) throw new Error("auth-callback-identity must bind immutable owner/profile and current generation");
  const postlock = ids.get("exchange-postlock-auth").text;
  for (const part of ["PAT", "membership", "workspace+machine+owner+profile+generation"]) {
    if (!postlock.includes(part)) throw new Error(`exchange-postlock-auth missing ${part}`);
  }
  const consumerTable = byName.get("task-consumers");
  if (!consumerTable) throw new Error("missing contract table: task-consumers");
  const declared = new Set();
  for (const row of consumerTable.rows) {
    const consumerPath = row.cells[1];
    const absolutePath = resolve(repoRoot, consumerPath);
    const relativePath = relative(repoRoot, absolutePath);
    if (relativePath.startsWith("..") || relativePath === "" || !statSync(absolutePath, { throwIfNoEntry: false })?.isFile()) {
      throw new Error(`missing task consumer: ${consumerPath}`);
    }
    const match = row.text.match(/(?:^|; )identities=([^ |]+)/);
    if (!match) continue;
    for (const identity of match[1].split(",")) declared.add(`${row.cells[1]}#${identity}`);
  }
  const discovered = new Set();
  walk(join(repoRoot, "server"), repoRoot, discovered);
  for (const identity of discovered) if (!declared.has(identity)) {
    const kind = identity.includes("#query:") ? "SQL" : "Go";
    throw new Error(`unclassified agent_task_queue ${kind} consumer: ${identity.replace("#query:", "#").replace("#go:", "#")}`);
  }
  for (const identity of declared) if (!discovered.has(identity)) throw new Error(`stale agent_task_queue consumer: ${identity}`);
  return { tables: tables.length, rows: ids.size, consumers: discovered.size };
}

try {
  const result = validate();
  process.stdout.write(`Twin contracts valid (${result.tables} tables, ${result.rows} rows, ${result.consumers} direct consumers).\n`);
} catch (error) {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
}
