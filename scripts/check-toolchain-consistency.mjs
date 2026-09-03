import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const failures = [];

function read(path) {
  return readFileSync(join(repoRoot, path), "utf8");
}

function readJSON(path) {
  return JSON.parse(read(path));
}

function capture(value, pattern, label) {
  const match = value.match(pattern);
  if (!match) {
    throw new Error(`Cannot derive ${label} from ${JSON.stringify(value)}`);
  }
  return match[1];
}

function requireSnippets(path, snippets) {
  const content = read(path);
  for (const snippet of snippets) {
    if (!content.includes(snippet)) {
      failures.push(`${path} must contain ${JSON.stringify(snippet)}`);
    }
  }
}

function requireMatches(path, pattern, expected, label, required = true) {
  const matches = [...read(path).matchAll(pattern)];
  if (required && matches.length === 0) {
    failures.push(`${path} must declare ${label}`);
    return;
  }
  for (const match of matches) {
    if (match[1] !== expected) {
      failures.push(`${path} declares ${label} ${match[1]}; expected ${expected}`);
    }
  }
}

function requireExact(path, expected) {
  const actual = read(path).trim();
  if (actual !== expected) {
    failures.push(`${path} is ${JSON.stringify(actual)}; expected ${JSON.stringify(expected)}`);
  }
}

const rootPackage = readJSON("package.json");
const mobilePackage = readJSON("apps/mobile/package.json");
const desktopPackage = readJSON("apps/desktop/package.json");
const webPackage = readJSON("apps/web/package.json");
const docsPackage = readJSON("apps/docs/package.json");

const nodeMajor = capture(rootPackage.engines.node, /^>=(\d+)$/, "Node major");
const pnpmVersion = capture(rootPackage.packageManager, /^pnpm@(.+)$/, "pnpm version");
const goVersion = capture(read("server/go.mod"), /^go (\d+\.\d+)(?:\.\d+)?$/m, "Go version");
const expoMajor = capture(mobilePackage.dependencies.expo, /^(?:[~^])?(\d+)/, "Expo major");
const reactVersion = capture(mobilePackage.dependencies.react, /^(\d+\.\d+)/, "React version");
const reactNativeVersion = capture(
  mobilePackage.dependencies["react-native"],
  /^(\d+\.\d+)/,
  "React Native version",
);
const expoRouterMajor = capture(
  mobilePackage.dependencies["expo-router"],
  /^(?:[~^])?(\d+)/,
  "Expo Router major",
);
const electronMajor = capture(desktopPackage.devDependencies.electron, /^(?:[~^])?(\d+)/, "Electron major");
const electronViteMajor = capture(
  desktopPackage.devDependencies["electron-vite"],
  /^(?:[~^])?(\d+)/,
  "electron-vite major",
);
const nextMajor = capture(webPackage.dependencies.next, /^(?:[~^])?(\d+)/, "Web Next.js major");
const docsNextMajor = capture(docsPackage.dependencies.next, /^(?:[~^])?(\d+)/, "Docs Next.js major");
const docsFumadocsVersion = docsPackage.dependencies["fumadocs-core"];
const docsFumadocsMdxVersion = docsPackage.dependencies["fumadocs-mdx"];
const desktopReactRouterVersion = desktopPackage.dependencies["react-router-dom"];

requireMatches("Dockerfile.web", /^FROM node:(\d+)-alpine(?: AS .+)?$/gm, nodeMajor, "Node major");
requireMatches("Dockerfile", /^FROM golang:(\d+\.\d+)-alpine(?: AS .+)?$/gm, goVersion, "Go version");

for (const filename of readdirSync(join(repoRoot, ".github/workflows"))) {
  if (!filename.endsWith(".yml") && !filename.endsWith(".yaml")) continue;
  const path = `.github/workflows/${filename}`;
  requireMatches(path, /^\s*node-version:\s*["']?(\d+)/gm, nodeMajor, "Node major", false);
  requireMatches(path, /^\s*go-version:\s*["']?(\d+\.\d+)/gm, goVersion, "Go version", false);
}

requireExact(".nvmrc", nodeMajor);
requireSnippets("scripts/dev.sh", [
  `Node.js ${nodeMajor}`,
  `pnpm ${pnpmVersion}`,
  `Go ${goVersion}`,
]);
requireSnippets("CLAUDE.md", [`CI runs Node ${nodeMajor}, the latest Go ${goVersion} patch`]);
requireSnippets("apps/mobile/CLAUDE.md", [
  `Expo SDK ${expoMajor}`,
  `React Native ${reactNativeVersion}`,
  `React ${reactVersion}`,
  `Expo Router ${expoRouterMajor}`,
]);
requireSnippets("SELF_HOSTING_WEB.md", [
  `Node.js ${nodeMajor}`,
  `pnpm ${pnpmVersion}`,
  `Go ${goVersion}`,
]);
requireSnippets("SELF_HOSTING_ADVANCED.md", [
  `Go ${goVersion}, Node.js ${nodeMajor}, pnpm ${pnpmVersion}`,
]);
requireSnippets("CONTRIBUTING.md", [
  `Node.js \`${nodeMajor}\``,
  `pnpm\` \`${pnpmVersion}\``,
  `Go \`${goVersion}\``,
]);

for (const suffix of ["", ".zh", ".ja", ".ko"]) {
  requireSnippets(`apps/docs/content/docs/developers/contributing${suffix}.mdx`, [
    `Node.js ${nodeMajor}`,
    `Go ${goVersion}`,
  ]);
}

requireSnippets("droid-wiki/overview/getting-started.md", [
  `Node.js ${nodeMajor}`,
  `Go ${goVersion}`,
]);
requireSnippets("droid-wiki/operations/local-development.md", [
  `Node.js ${nodeMajor}`,
  `Go ${goVersion}`,
]);
requireSnippets("droid-wiki/operations/deployment-and-self-hosting.md", [
  `Go ${goVersion} Alpine builder`,
  `Node ${nodeMajor}`,
]);
requireSnippets("droid-wiki/operations/ci-testing-and-release.md", [
  `Node ${nodeMajor}`,
  `Go ${goVersion}.x`,
]);
requireSnippets("droid-wiki/reference/test-map.md", [
  `Node ${nodeMajor}`,
  `Go ${goVersion}.x`,
]);
requireSnippets("droid-wiki/apps/mobile.md", [
  `| Expo SDK | ${expoMajor} |`,
  `| React Native | ${mobilePackage.dependencies["react-native"]} |`,
]);
requireSnippets("droid-wiki/apps/index.md", [
  `Electron ${electronMajor}`,
  `Expo SDK ${expoMajor}`,
  `Next.js ${docsNextMajor}`,
]);
requireSnippets("droid-wiki/apps/cross-platform-matrix.md", [
  `Next ${nextMajor}`,
  `Electron ${electronMajor} + electron-vite ${electronViteMajor}`,
  `Expo ${expoMajor} + RN ${mobilePackage.dependencies["react-native"]}`,
  `Next ${docsNextMajor} + Fumadocs`,
]);
requireSnippets("droid-wiki/apps/desktop.md", [
  `Electron \`${desktopPackage.devDependencies.electron}\``,
  `electron-vite \`${desktopPackage.devDependencies["electron-vite"]}\``,
  `React Router \`${desktopReactRouterVersion}\``,
]);
requireSnippets("droid-wiki/apps/docs-site.md", [
  `Next.js \`${docsPackage.dependencies.next}\``,
  `Fumadocs Core/UI \`${docsFumadocsVersion}\``,
  `Fumadocs MDX \`${docsFumadocsMdxVersion}\``,
]);

if (failures.length > 0) {
  console.error("Toolchain version consistency check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(
  `Toolchain versions are consistent: Node ${nodeMajor}, pnpm ${pnpmVersion}, Go ${goVersion}, Expo ${expoMajor}, React Native ${reactNativeVersion}.`,
);
