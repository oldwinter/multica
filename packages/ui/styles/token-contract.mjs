import {
  SEMANTIC_CONTRAST_REQUIREMENTS,
  SEMANTIC_TOKEN_CONTRACT_VERSION,
} from "../../core/constants/semantic-token-schema.ts";

const CSS_TOKEN_BY_SEMANTIC_ROLE = Object.freeze({
  canvas: "--page-canvas",
  shell: "--app-shell",
  surface: "--surface",
  raisedSurface: "--surface-raised",
  surfaceHover: "--surface-hover",
  selection: "--surface-selected",
  selectionForeground: "--surface-selected-foreground",
  border: "--surface-border",
  controlBorder: "--control-border",
  text: "--foreground",
  mutedText: "--muted-foreground",
  disabledText: "--disabled-foreground",
  focus: "--ring",
  brand: "--brand",
  brandForeground: "--brand-foreground",
  destructive: "--destructive",
  destructiveForeground: "--destructive-foreground",
  success: "--success",
  warning: "--warning",
  info: "--info",
  statusBacklog: "--status-backlog",
  statusTodo: "--status-todo",
  statusInProgress: "--status-in-progress",
  statusDone: "--status-done",
  statusCancelled: "--status-cancelled",
  chart1: "--chart-1",
  chart2: "--chart-2",
  chart3: "--chart-3",
  chart4: "--chart-4",
  chart5: "--chart-5",
  codeSurface: "--code-background",
  codeText: "--code-foreground",
  platformChrome: "--platform-chrome",
  platformChromeForeground: "--platform-chrome-foreground",
});

const COMPONENT_TEXT_CONTRAST_REQUIREMENTS = [
  ["surface-text", "--surface-foreground", "--surface", 4.5],
  ["card-text", "--card-foreground", "--card", 4.5],
  ["popover-text", "--popover-foreground", "--popover", 4.5],
  ["primary-text", "--primary-foreground", "--primary", 4.5],
  ["secondary-text", "--secondary-foreground", "--secondary", 4.5],
  ["accent-text", "--accent-foreground", "--accent", 4.5],
  ["sidebar-text", "--sidebar-foreground", "--sidebar", 4.5],
  ["sidebar-primary-text", "--sidebar-primary-foreground", "--sidebar-primary", 4.5],
].map(([id, foreground, background, minimum]) => ({
  id,
  category: "text",
  foreground,
  background,
  minimum,
}));

export const APPEARANCE_TOKEN_CONTRACT = Object.freeze({
  version: SEMANTIC_TOKEN_CONTRACT_VERSION,
  skins: Object.freeze(["tension", "relay", "field"]),
  modes: Object.freeze(["light", "dark"]),
  roleTokens: CSS_TOKEN_BY_SEMANTIC_ROLE,
  statusTokens: Object.freeze([
    "--status-backlog",
    "--status-todo",
    "--status-in-progress",
    "--status-done",
    "--status-cancelled",
  ]),
  tokens: Object.freeze([
    ...new Set([
      ...Object.values(CSS_TOKEN_BY_SEMANTIC_ROLE),
      "--app-shell",
      "--page-canvas",
      "--surface",
      "--surface-foreground",
      "--surface-raised",
      "--surface-hover",
      "--surface-selected",
      "--surface-selected-foreground",
      "--surface-border",
      "--background",
      "--foreground",
      "--card",
      "--card-foreground",
      "--popover",
      "--popover-foreground",
      "--primary",
      "--primary-foreground",
      "--secondary",
      "--secondary-foreground",
      "--muted",
      "--muted-foreground",
      "--faint-foreground",
      "--accent",
      "--accent-foreground",
      "--destructive",
      "--border",
      "--input",
      "--ring",
      "--brand",
      "--brand-foreground",
      "--success",
      "--warning",
      "--info",
      "--chart-1",
      "--chart-2",
      "--chart-3",
      "--chart-4",
      "--chart-5",
      "--sidebar",
      "--sidebar-foreground",
      "--sidebar-primary",
      "--sidebar-primary-foreground",
      "--sidebar-accent",
      "--sidebar-accent-foreground",
      "--sidebar-border",
      "--sidebar-ring",
      "--overlay-backdrop",
      "--code-background",
      "--code-foreground",
      "--editor-selection",
      "--editor-selection-foreground",
      "--platform-chrome",
      "--platform-chrome-foreground",
      "--status-backlog",
      "--status-todo",
      "--status-in-progress",
      "--status-done",
      "--status-cancelled",
    ]),
  ]),
  contrastPairs: Object.freeze(
    [
      ...SEMANTIC_CONTRAST_REQUIREMENTS.map((requirement) => ({
        ...requirement,
        foreground: CSS_TOKEN_BY_SEMANTIC_ROLE[requirement.foreground],
        background: CSS_TOKEN_BY_SEMANTIC_ROLE[requirement.background],
      })),
      ...COMPONENT_TEXT_CONTRAST_REQUIREMENTS,
    ],
  ),
});

// These are debt ceilings, not allowances. When a raw color is migrated from
// Docs or the public landing surface, lower the matching number in the same
// change so later work cannot silently spend the reduction again.
export const RAW_COLOR_DEBT_BUDGETS = Object.freeze({
  "apps/docs": 11,
  "apps/web/features/landing": 10,
});

const RAW_COLOR_EXCEPTIONS = new Map([
  ["components/ui/chart.tsx", new Map([["#ccc", 3], ["#fff", 2]])],
  [
    "components/ui/dot-sphere.tsx",
    new Map([["rgba(129, 140, 248, 0.5)", 1], ["oklch(0.145 0 0)", 1]]),
  ],
  ["styles/base.css", new Map([["#000", 2]])],
]);

const SKIN_BRANCH_ADAPTERS = new Set([
  "components/common/theme-provider.tsx",
  "styles/token-contract.mjs",
]);

function isSkinBranchAdapter(sourcePath) {
  return [...SKIN_BRANCH_ADAPTERS].some(
    (adapterPath) => sourcePath === adapterPath || sourcePath.endsWith(`/${adapterPath}`),
  );
}

function stripComments(css) {
  return css.replace(/\/\*[\s\S]*?\*\//g, "");
}

function declarationBlocks(css) {
  const blocks = new Map();
  const source = stripComments(css);
  const blockPattern = /([^{}]+)\{([^{}]*)\}/g;
  let blockMatch;

  while ((blockMatch = blockPattern.exec(source)) !== null) {
    const selector = blockMatch[1].trim();
    const declarations = new Map();
    const declarationPattern = /(--[a-z0-9-]+)\s*:\s*([^;]+);/gi;
    let declarationMatch;
    while ((declarationMatch = declarationPattern.exec(blockMatch[2])) !== null) {
      declarations.set(declarationMatch[1], declarationMatch[2].trim());
    }
    if (declarations.size > 0) blocks.set(selector, declarations);
  }

  return blocks;
}

function declarationsFor(blocks, skin, mode) {
  const selectors = [":root"];
  if (mode === "dark") selectors.push(".dark");
  if (skin !== "tension") {
    selectors.push(`:root[data-skin="${skin}"]`);
    if (mode === "dark") selectors.push(`.dark[data-skin="${skin}"]`);
  }

  const declarations = new Map();
  for (const selector of selectors) {
    for (const [name, value] of blocks.get(selector) ?? []) {
      declarations.set(name, value);
    }
  }
  return declarations;
}

function references(value) {
  return [...value.matchAll(/var\(\s*(--[a-z0-9-]+)/gi)].map((match) => match[1]);
}

function resolveToken(name, declarations, stack = []) {
  if (stack.includes(name)) {
    return { cycle: [...stack.slice(stack.indexOf(name)), name] };
  }
  const value = declarations.get(name);
  if (value === undefined) return { missing: name };

  let resolved = value;
  for (const reference of references(value)) {
    const result = resolveToken(reference, declarations, [...stack, name]);
    if (result.cycle || result.missing) return result;
    resolved = resolved.replace(
      new RegExp(`var\\(\\s*${reference.replaceAll("-", "\\-")}\\s*\\)`),
      result.value,
    );
  }
  return { value: resolved };
}

function oklchToRgb(value) {
  const match = value.match(
    /^oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+)%?)?\s*\)$/,
  );
  if (!match) return null;

  const lightness = Number(match[1]);
  const chroma = Number(match[2]);
  const hue = (Number(match[3]) * Math.PI) / 180;
  const alpha = match[4] === undefined ? 1 : Number(match[4]) / (value.includes("%") ? 100 : 1);
  if (alpha < 1) return null;

  const a = chroma * Math.cos(hue);
  const b = chroma * Math.sin(hue);
  const lPrime = lightness + 0.3963377774 * a + 0.2158037573 * b;
  const mPrime = lightness - 0.1055613458 * a - 0.0638541728 * b;
  const sPrime = lightness - 0.0894841775 * a - 1.291485548 * b;
  const l = lPrime ** 3;
  const m = mPrime ** 3;
  const s = sPrime ** 3;

  const linear = [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ];
  return linear.map((channel) => Math.min(1, Math.max(0, channel)));
}

function relativeLuminance(rgb) {
  return 0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2];
}

function contrastRatio(foreground, background) {
  const foregroundRgb = oklchToRgb(foreground);
  const backgroundRgb = oklchToRgb(background);
  if (!foregroundRgb || !backgroundRgb) return null;
  const lighter = Math.max(relativeLuminance(foregroundRgb), relativeLuminance(backgroundRgb));
  const darker = Math.min(relativeLuminance(foregroundRgb), relativeLuminance(backgroundRgb));
  return (lighter + 0.05) / (darker + 0.05);
}

export function auditAppearanceTokens({ tokensCss }) {
  const blocks = declarationBlocks(tokensCss);
  const combinations = [];
  const missingTokens = [];
  const invalidReferences = [];
  const cycles = [];
  const contrastFailures = [];
  const statusCollisions = [];

  for (const skin of APPEARANCE_TOKEN_CONTRACT.skins) {
    for (const mode of APPEARANCE_TOKEN_CONTRACT.modes) {
      const declarations = declarationsFor(blocks, skin, mode);
      const resolved = new Map();
      combinations.push({ skin, mode });

      for (const token of APPEARANCE_TOKEN_CONTRACT.tokens) {
        const result = resolveToken(token, declarations);
        if (result.cycle) {
          cycles.push({ skin, mode, token, cycle: result.cycle });
        } else if (result.missing) {
          const failure = { skin, mode, token, reference: result.missing };
          if (result.missing === token) missingTokens.push(failure);
          else invalidReferences.push(failure);
        } else {
          resolved.set(token, result.value);
        }
      }

      for (const {
        id,
        category,
        foreground,
        background,
        minimum,
      } of APPEARANCE_TOKEN_CONTRACT.contrastPairs) {
        const foregroundValue = resolved.get(foreground);
        const backgroundValue = resolved.get(background);
        if (!foregroundValue || !backgroundValue) continue;
        const ratio = contrastRatio(foregroundValue, backgroundValue);
        if (ratio === null || ratio + Number.EPSILON < minimum) {
          contrastFailures.push({ skin, mode, id, category, foreground, background, minimum, ratio });
        }
      }

      for (const [index, token] of APPEARANCE_TOKEN_CONTRACT.statusTokens.entries()) {
        const value = resolved.get(token);
        if (!value) continue;
        for (const comparedToken of APPEARANCE_TOKEN_CONTRACT.statusTokens.slice(index + 1)) {
          if (resolved.get(comparedToken) === value) {
            statusCollisions.push({ skin, mode, tokens: [token, comparedToken], value });
          }
        }
      }
    }
  }

  return {
    combinations,
    missingTokens,
    invalidReferences,
    cycles,
    contrastFailures,
    statusCollisions,
  };
}

function rawColors(source) {
  const colors = stripSourceComments(source).match(
    /#[0-9a-f]{3,8}\b|\b(?:rgb|rgba|hsl|hsla|oklch|oklab)\([^)]*\)/gi,
  );
  return (colors ?? []).filter((value) => !value.includes("var(--"));
}

function stripSourceComments(source) {
  return stripComments(source).replace(/(^|\s)\/\/.*$/gm, "$1");
}

function skinBranches(source) {
  const branches = [];
  const productComparison = /\bskin\s*(?:===|!==)\s*["'](tension|relay|field)["']/gi;
  const switchCase = /\bcase\s+["'](tension|relay|field)["']\s*:/gi;
  let match;
  while ((match = productComparison.exec(source)) !== null) branches.push(match[1]);
  while ((match = switchCase.exec(source)) !== null) branches.push(match[1]);
  return branches;
}

function policyHasPath(paths, sourcePath) {
  return paths?.has(sourcePath) ?? false;
}

function debtFor(sourcePath, rawColorPolicy) {
  const policyDebt = rawColorPolicy?.debt?.get(sourcePath);
  if (policyDebt) return policyDebt;
  for (const [exceptionPath, allowance] of RAW_COLOR_EXCEPTIONS) {
    if (sourcePath === exceptionPath || sourcePath.endsWith(`/${exceptionPath}`)) return allowance;
  }
  return new Map();
}

export function auditProductSources({ sourceFiles, checkRawColors = true, rawColorPolicy }) {
  const rawColorViolations = [];
  const skinBranchViolations = [];

  for (const [sourcePath, source] of sourceFiles) {
    const normalizedPath = sourcePath.replaceAll("\\", "/");
    if (
      checkRawColors &&
      normalizedPath !== "styles/tokens.css" &&
      !normalizedPath.endsWith("/styles/tokens.css") &&
      normalizedPath !== "styles/token-contract.mjs" &&
      !normalizedPath.endsWith("/styles/token-contract.mjs") &&
      normalizedPath !== "styles/raw-color-policy.mjs" &&
      !normalizedPath.endsWith("/styles/raw-color-policy.mjs") &&
      !policyHasPath(rawColorPolicy?.approvedSources, normalizedPath) &&
      !(rawColorPolicy?.ignoredPrefixes ?? []).some((prefix) => normalizedPath.startsWith(prefix))
    ) {
      const exceptions = debtFor(normalizedPath, rawColorPolicy);
      const consumed = new Map();
      for (const value of rawColors(source)) {
        const nextCount = (consumed.get(value) ?? 0) + 1;
        consumed.set(value, nextCount);
        if (nextCount > (exceptions.get(value) ?? 0)) {
          rawColorViolations.push({ path: normalizedPath, value });
        }
      }
    }

    if (!isSkinBranchAdapter(normalizedPath) && /\.[cm]?[jt]sx?$/.test(normalizedPath)) {
      for (const skin of skinBranches(stripSourceComments(source))) {
        skinBranchViolations.push({ path: normalizedPath, skin });
      }
    }
  }

  return { rawColorViolations, skinBranchViolations };
}

export function auditRawColorDebt({ sourceFiles, budgets }) {
  const summaries = [];
  const overages = [];

  for (const [scope, budget] of Object.entries(budgets)) {
    const scopedSources = new Map(
      [...sourceFiles].filter(
        ([sourcePath]) => sourcePath === scope || sourcePath.startsWith(`${scope}/`),
      ),
    );
    const violations = auditProductSources({
      sourceFiles: scopedSources,
      checkRawColors: true,
    }).rawColorViolations;
    const summary = { scope, budget, count: violations.length };
    summaries.push(summary);
    if (violations.length > budget) {
      overages.push({ ...summary, violations });
    }
  }

  return { summaries, overages };
}

export function auditAccessibilityGuards({ baseCss }) {
  const checks = [
    ["forced-colors media query", /@media\s*\(forced-colors:\s*active\)/],
    ["forced-colors canvas", /--background:\s*Canvas\s*;/],
    ["forced-colors text", /--foreground:\s*CanvasText\s*;/],
    ["forced-colors selection", /--surface-selected:\s*Highlight\s*;/],
    ["forced-colors selection text", /--surface-selected-foreground:\s*HighlightText\s*;/],
    ["forced-colors focus", /--ring:\s*Highlight\s*;/],
    ["forced-colors selected outline", /outline:\s*2px\s+solid\s+Highlight\s*;/],
    ["reduced-motion media query", /@media\s*\(prefers-reduced-motion:\s*reduce\)/],
    [
      "reduced-motion view transition",
      /html\[data-theme-transition="reveal"\]::view-transition-new\(root\)\s*\{\s*animation:\s*none\s*;/,
    ],
  ];

  return {
    missingGuards: checks
      .filter(([, pattern]) => !pattern.test(baseCss))
      .map(([name]) => name),
  };
}
