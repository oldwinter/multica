import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { fileURLToPath, URL } from "node:url";

import {
  APPEARANCE_TOKEN_CONTRACT,
  RAW_COLOR_DEBT_BUDGETS,
  auditAccessibilityGuards,
  auditAppearanceTokens,
  auditProductSources,
  auditRawColorDebt,
} from "./token-contract.mjs";
import { PRODUCT_RAW_COLOR_POLICY } from "./raw-color-policy.mjs";

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

const repoRoot = fileURLToPath(new URL("../../../", import.meta.url));
const productRoots = [
  "packages/ui",
  "packages/views",
  "apps/web",
  "apps/desktop",
  "apps/mobile",
];
const branchOnlyRoots = ["apps/docs"];
const [tokensCss, baseCss, ...productSourceGroups] = await Promise.all([
  readFile(new URL("./tokens.css", import.meta.url), "utf8"),
  readFile(new URL("./base.css", import.meta.url), "utf8"),
  ...productRoots.map((root) => sourceFiles(path.join(repoRoot, root), repoRoot)),
]);
const productSources = new Map(productSourceGroups.flatMap((group) => [...group]));
const branchOnlySources = new Map(
  (await Promise.all(branchOnlyRoots.map((root) => sourceFiles(path.join(repoRoot, root), repoRoot)))).flatMap(
    (group) => [...group],
  ),
);
const allProductSources = new Map([...productSources, ...branchOnlySources]);

const tokenReport = auditAppearanceTokens({ tokensCss });
const productReport = auditProductSources({
  sourceFiles: productSources,
  rawColorPolicy: PRODUCT_RAW_COLOR_POLICY,
});
const branchOnlyReport = auditProductSources({
  sourceFiles: branchOnlySources,
  checkRawColors: false,
});
const accessibilityReport = auditAccessibilityGuards({ baseCss });
const rawColorDebtReport = auditRawColorDebt({
  sourceFiles: allProductSources,
  budgets: RAW_COLOR_DEBT_BUDGETS,
});
const failures = {
  missingTokens: tokenReport.missingTokens,
  invalidReferences: tokenReport.invalidReferences,
  cycles: tokenReport.cycles,
  contrastFailures: tokenReport.contrastFailures,
  statusCollisions: tokenReport.statusCollisions,
  rawColorViolations: productReport.rawColorViolations,
  skinBranchViolations: [
    ...productReport.skinBranchViolations,
    ...branchOnlyReport.skinBranchViolations,
  ],
  rawColorDebtOverages: rawColorDebtReport.overages,
  missingGuards: accessibilityReport.missingGuards,
};

if (Object.values(failures).some((failure) => failure.length > 0)) {
  process.stderr.write(`${JSON.stringify(failures, null, 2)}\n`);
  process.exitCode = 1;
} else {
  process.stdout.write(
    `Appearance token contract v${APPEARANCE_TOKEN_CONTRACT.version}: ` +
      `${tokenReport.combinations.length} skin/mode combinations passed; ` +
      `raw-color debt ${rawColorDebtReport.summaries
        .map(({ scope, count, budget }) => `${scope} ${count}/${budget}`)
        .join(", ")}\n`,
  );
}
