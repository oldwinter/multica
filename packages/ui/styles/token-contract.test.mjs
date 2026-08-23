import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { URL } from "node:url";

import {
  SEMANTIC_CONTRAST_REQUIREMENTS,
  SEMANTIC_TOKEN_CONTRACT_VERSION,
  SEMANTIC_TOKEN_ROLES,
} from "../../core/constants/semantic-token-schema.ts";

import {
  APPEARANCE_TOKEN_CONTRACT,
  auditAccessibilityGuards,
  auditAppearanceTokens,
  auditProductSources,
  auditRawColorDebt,
} from "./token-contract.mjs";
import { PRODUCT_RAW_COLOR_POLICY } from "./raw-color-policy.mjs";

const tokensCss = await readFile(new URL("./tokens.css", import.meta.url), "utf8");
const baseCss = await readFile(new URL("./base.css", import.meta.url), "utf8");

test("resolves the versioned semantic contract for every skin and mode", () => {
  const report = auditAppearanceTokens({ tokensCss });

  assert.equal(APPEARANCE_TOKEN_CONTRACT.version, SEMANTIC_TOKEN_CONTRACT_VERSION);
  assert.deepEqual(Object.keys(APPEARANCE_TOKEN_CONTRACT.roleTokens), SEMANTIC_TOKEN_ROLES);
  assert.deepEqual(
    [...new Set(SEMANTIC_CONTRAST_REQUIREMENTS.map(({ category }) => category))].sort(),
    [
      "charts",
      "code",
      "controlBorders",
      "destructive",
      "disabled",
      "focus",
      "selection",
      "statusGraphics",
      "text",
    ],
  );
  assert.deepEqual(
    report.combinations.map(({ skin, mode }) => `${skin}/${mode}`),
    [
      "tension/light",
      "tension/dark",
      "relay/light",
      "relay/dark",
      "field/light",
      "field/dark",
    ],
  );
  assert.deepEqual(report.missingTokens, []);
  assert.deepEqual(report.invalidReferences, []);
  assert.deepEqual(report.cycles, []);
  assert.deepEqual(report.contrastFailures, []);
  assert.deepEqual(report.statusCollisions, []);
});

test("reports broken token graphs instead of silently accepting them", () => {
  const missing = auditAppearanceTokens({
    tokensCss: tokensCss.replace("    --status-cancelled: var(--faint-foreground);\n", ""),
  });
  assert.equal(
    missing.missingTokens.filter(({ token }) => token === "--status-cancelled").length,
    6,
  );

  const invalid = auditAppearanceTokens({
    tokensCss: tokensCss.replace(
      "--status-cancelled: var(--faint-foreground);",
      "--status-cancelled: var(--unknown-status);",
    ),
  });
  assert.equal(
    invalid.invalidReferences.filter(({ reference }) => reference === "--unknown-status").length,
    6,
  );

  const cyclic = auditAppearanceTokens({
    tokensCss: tokensCss
      .replace("--status-backlog: var(--muted-foreground);", "--status-backlog: var(--status-cancelled);")
      .replace("--status-cancelled: var(--faint-foreground);", "--status-cancelled: var(--status-backlog);"),
  });
  assert.ok(cyclic.cycles.some(({ token }) => token === "--status-backlog"));
  assert.ok(cyclic.cycles.some(({ token }) => token === "--status-cancelled"));

  const unmeasurable = auditAppearanceTokens({
    tokensCss: tokensCss.replaceAll(/--ring:\s*oklch\([^;]+\);/g, "--ring: currentColor;"),
  });
  assert.equal(
    unmeasurable.contrastFailures.filter(({ id, ratio }) => id === "focus-on-canvas" && ratio === null)
      .length,
    6,
  );

  const collided = auditAppearanceTokens({
    tokensCss: tokensCss.replace(
      "--status-cancelled: var(--faint-foreground);",
      "--status-cancelled: var(--muted-foreground);",
    ),
  });
  assert.equal(collided.statusCollisions.length, 6);
});

async function sourceFiles(directory, root = directory) {
  const files = new Map();
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (
      [
        "node_modules",
        "__tests__",
        ".next",
        ".turbo",
        ".source",
        ".expo",
        "coverage",
        "dist",
        "build",
        "out",
      ].includes(entry.name)
    ) {
      continue;
    }
    const absolutePath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      for (const [name, content] of await sourceFiles(absolutePath, root)) {
        files.set(name, content);
      }
    } else if (
      /\.(?:css|[cm]?js|jsx|ts|tsx)$/.test(entry.name) &&
      !/\.(?:test|spec)\.[cm]?[jt]sx?$/.test(entry.name)
    ) {
      files.set(path.relative(root, absolutePath), await readFile(absolutePath, "utf8"));
    }
  }
  return files;
}

test("rejects raw product colors and skin-specific product behavior", async () => {
  const fixture = new Map([
    ["components/example.tsx", 'const tone = skin === "relay" ? "bg-[#123456]" : "bg-background";'],
    ["styles/tokens.css", ":root { --brand: oklch(0.5 0.2 30); }"],
  ]);
  const fixtureReport = auditProductSources({ sourceFiles: fixture });

  assert.deepEqual(fixtureReport.rawColorViolations, [
    { path: "components/example.tsx", value: "#123456" },
  ]);
  assert.deepEqual(fixtureReport.skinBranchViolations, [
    { path: "components/example.tsx", skin: "relay" },
  ]);

  const allowedOnce = auditProductSources({
    sourceFiles: new Map([["components/debt.tsx", 'const value = "#123456";']]),
    rawColorPolicy: {
      approvedSources: new Set(),
      debt: new Map([["components/debt.tsx", new Map([["#123456", 1]])]]),
    },
  });
  assert.deepEqual(allowedOnce.rawColorViolations, []);

  const duplicatedDebt = auditProductSources({
    sourceFiles: new Map([
      ["components/debt.tsx", 'const first = "#123456"; const second = "#123456";'],
    ]),
    rawColorPolicy: {
      approvedSources: new Set(),
      debt: new Map([["components/debt.tsx", new Map([["#123456", 1]])]]),
    },
  });
  assert.deepEqual(duplicatedDebt.rawColorViolations, [
    { path: "components/debt.tsx", value: "#123456" },
  ]);

  const packageRoot = new URL("..", import.meta.url);
  const report = auditProductSources({ sourceFiles: await sourceFiles(packageRoot.pathname) });
  assert.deepEqual(report.rawColorViolations, []);
  assert.deepEqual(report.skinBranchViolations, []);
});

test("keeps raw-color migration debt on a ratcheting budget", () => {
  const sourceFiles = new Map([
    ["apps/docs/first.tsx", 'const first = "#123456";'],
    ["apps/docs/second.tsx", 'const second = "rgb(1, 2, 3)";'],
    ["apps/web/features/landing/clean.tsx", "export const clean = true;"],
  ]);
  const report = auditRawColorDebt({
    sourceFiles,
    budgets: {
      "apps/docs": 1,
      "apps/web/features/landing": 0,
    },
  });

  assert.deepEqual(report.summaries, [
    { scope: "apps/docs", budget: 1, count: 2 },
    { scope: "apps/web/features/landing", budget: 0, count: 0 },
  ]);
  assert.equal(report.overages.length, 1);
  assert.equal(report.overages[0].scope, "apps/docs");
  assert.equal(report.overages[0].violations.length, 2);
});

test("keeps approved product raw-color debt at or below its recorded count", async () => {
  const repoRoot = new URL("../../../", import.meta.url);
  const roots = ["packages/views", "apps/web", "apps/desktop", "apps/mobile"];
  const productSources = new Map();
  for (const root of roots) {
    for (const [name, content] of await sourceFiles(new URL(`${root}/`, repoRoot).pathname, repoRoot.pathname)) {
      productSources.set(name, content);
    }
  }
  const report = auditProductSources({
    sourceFiles: productSources,
    rawColorPolicy: PRODUCT_RAW_COLOR_POLICY,
  });
  assert.deepEqual(report.rawColorViolations, []);
});

test("keeps forced-colors and reduced-motion behavior inside the shared contract", () => {
  const report = auditAccessibilityGuards({ baseCss });

  assert.deepEqual(report.missingGuards, []);
});
