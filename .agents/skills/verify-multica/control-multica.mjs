#!/usr/bin/env node
/**
 * Verification harness for this checkout's Multica web app.
 * Invocation is documented in SKILL.md. Do not drive an instance whose
 * /health pid+commit do not belong to this checkout.
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { execSync } from "node:child_process";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../..");
const artifactsRoot = path.join(here, "artifacts");

function loadEnvFile(filePath) {
  if (!existsSync(filePath)) return;
  const text = readFileSync(filePath, "utf8");
  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;
    const eq = line.indexOf("=");
    if (eq <= 0) continue;
    const key = line.slice(0, eq).trim();
    let value = line.slice(eq + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    if (process.env[key] === undefined || process.env[key] === "") {
      process.env[key] = value;
    }
  }
}

function loadCheckoutEnv() {
  const worktree = path.join(repoRoot, ".env.worktree");
  const main = path.join(repoRoot, ".env");
  if (existsSync(worktree)) loadEnvFile(worktree);
  else loadEnvFile(main);
}

function backendPort() {
  return (
    process.env.BACKEND_PORT ||
    process.env.API_PORT ||
    process.env.SERVER_PORT ||
    process.env.PORT ||
    "8080"
  );
}

function frontendPort() {
  return process.env.FRONTEND_PORT || "3000";
}

function apiBase() {
  return process.env.NEXT_PUBLIC_API_URL || `http://localhost:${backendPort()}`;
}

function frontendOrigin() {
  return (
    process.env.PLAYWRIGHT_BASE_URL ||
    process.env.FRONTEND_ORIGIN ||
    `http://localhost:${frontendPort()}`
  );
}

function databaseUrl() {
  return (
    process.env.DATABASE_URL ||
    "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
  );
}

function gitShortHead() {
  try {
    return execSync("git rev-parse --short HEAD", {
      cwd: repoRoot,
      encoding: "utf8",
    }).trim();
  } catch {
    return "unknown";
  }
}

async function fetchJson(url, init = {}, timeoutMs = 5000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(url, { ...init, signal: controller.signal });
    const text = await res.text();
    let body = null;
    if (text) {
      try {
        body = JSON.parse(text);
      } catch {
        body = text;
      }
    }
    return { ok: res.ok, status: res.status, body };
  } catch (error) {
    return { ok: false, status: 0, body: null, error: String(error) };
  } finally {
    clearTimeout(timer);
  }
}

function print(obj) {
  process.stdout.write(`${JSON.stringify(obj, null, 2)}\n`);
}

function fail(message, extra = {}) {
  print({ ok: false, error: message, ...extra });
  process.exit(1);
}

async function doctor() {
  loadCheckoutEnv();
  const expectedCommit = gitShortHead();
  const api = apiBase();
  const origin = frontendOrigin();
  const health = await fetchJson(`${api}/health`);
  const ready = await fetchJson(`${api}/healthz`);
  let frontend = { ok: false, status: 0 };
  try {
    frontend = await fetchJson(origin, {}, 8000);
  } catch (error) {
    frontend = { ok: false, status: 0, error: String(error) };
  }

  const healthBody = health.body && typeof health.body === "object" ? health.body : {};
  const readyBody = ready.body && typeof ready.body === "object" ? ready.body : {};
  const reportedCommit = typeof healthBody.commit === "string" ? healthBody.commit : "";
  const commitMatches =
    reportedCommit !== "" &&
    reportedCommit !== "unknown" &&
    (reportedCommit === expectedCommit || expectedCommit.startsWith(reportedCommit));

  const result = {
    ok: false,
    checkout: repoRoot,
    expected_commit: expectedCommit,
    api,
    origin,
    health: {
      status: health.status,
      body: healthBody,
      error: health.error ?? null,
    },
    ready: {
      status: ready.status,
      body: readyBody,
      error: ready.error ?? null,
    },
    frontend: {
      status: frontend.status,
      ok: frontend.ok === true,
      error: frontend.error ?? null,
    },
    identity: {
      pid: healthBody.pid ?? null,
      commit: reportedCommit || null,
      started_at: healthBody.started_at ?? null,
      commit_matches_checkout: commitMatches,
    },
  };

  const apiLive = health.ok === true && healthBody.status === "ok";
  const dbReady = ready.ok === true && readyBody?.checks?.db === "ok";
  const migrationsReady = ready.ok === true && readyBody?.checks?.migrations === "ok";
  const webUp = frontend.ok === true;

  if (!apiLive) {
    result.error =
      "API /health is not ok. Run `make up` for this checkout, or refuse to drive a foreign instance.";
    print(result);
    process.exit(1);
  }
  if (!dbReady || !migrationsReady) {
    result.error = "API /healthz is not ready (db or migrations).";
    print(result);
    process.exit(1);
  }
  if (!webUp) {
    result.error = `Frontend at ${origin} is not answering.`;
    print(result);
    process.exit(1);
  }
  if (reportedCommit && reportedCommit !== "unknown" && !commitMatches) {
    result.error =
      "API /health commit does not match this checkout. Refuse to drive it; it is not this environment.";
    print(result);
    process.exit(1);
  }
  if (!reportedCommit || reportedCommit === "unknown") {
    result.warning =
      "API /health commit is unknown. Identity is weaker than `make up` (ldflag). Continue only if you started this process from this checkout.";
  }

  result.ok = true;
  print(result);
}

function randomId() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

async function authedFetch(token, pathName, init = {}, workspaceId, workspaceSlug) {
  const headers = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
    ...(init.headers ?? {}),
  };
  if (workspaceSlug) headers["X-Workspace-Slug"] = workspaceSlug;
  else if (workspaceId) headers["X-Workspace-ID"] = workspaceId;
  return fetchJson(`${apiBase()}${pathName}`, { ...init, headers }, 15000);
}

async function login(opts = {}) {
  loadCheckoutEnv();
  const runId = opts.runId || randomId();
  const email = opts.email || `verify-${runId}@multica.local`;
  const name = opts.name || "Verify User";
  const workspaceName = opts.workspaceName || `Verify ${runId}`;
  const workspaceSlug = opts.workspaceSlug || `verify-${runId}`;
  const code =
    process.env.MULTICA_DEV_VERIFICATION_CODE?.trim() || "888888";

  const send = await fetchJson(`${apiBase()}/auth/send-code`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
  if (!send.ok) {
    fail("send-code failed", { status: send.status, body: send.body });
  }

  const verify = await fetchJson(`${apiBase()}/auth/verify-code`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, code }),
  });
  const token = verify.body?.token;
  if (!verify.ok || !token) {
    fail(
      "verify-code failed. Do not retry immediately — repeated attempts lock the code. Wait ~40s.",
      { status: verify.status, body: verify.body },
    );
  }

  await authedFetch(token, "/api/me", {
    method: "PATCH",
    body: JSON.stringify({ name, language: "en" }),
  });

  const workspacesRes = await authedFetch(token, "/api/workspaces");
  const workspaces = Array.isArray(workspacesRes.body) ? workspacesRes.body : [];
  let workspace =
    workspaces.find((item) => item.slug === workspaceSlug) ?? workspaces[0];
  if (!workspace) {
    const created = await authedFetch(token, "/api/workspaces", {
      method: "POST",
      body: JSON.stringify({ name: workspaceName, slug: workspaceSlug }),
    });
    if (!created.ok) {
      fail("workspace create failed", { status: created.status, body: created.body });
    }
    workspace = created.body;
  }

  await authedFetch(
    token,
    "/api/me/onboarding/complete",
    {
      method: "POST",
      body: JSON.stringify({ exit: "existing" }),
    },
    workspace.id,
    workspace.slug,
  );

  const session = {
    ok: true,
    run_id: runId,
    email,
    token,
    workspace_id: workspace.id,
    workspace_slug: workspace.slug,
    origin: frontendOrigin(),
    api: apiBase(),
    created_issue_ids: [],
    started_env: opts.startedEnv === true,
  };
  return session;
}

function sessionPath(runId) {
  return path.join(artifactsRoot, runId, "session.json");
}

function writeSession(session) {
  const dir = path.join(artifactsRoot, session.run_id);
  mkdirSync(dir, { recursive: true });
  writeFileSync(sessionPath(session.run_id), `${JSON.stringify(session, null, 2)}\n`);
}

function readSession(runId) {
  const file = sessionPath(runId);
  if (!existsSync(file)) fail(`No session.json for run ${runId}`);
  return JSON.parse(readFileSync(file, "utf8"));
}

async function cmdLogin() {
  const startedEnv = process.env.VERIFY_STARTED_ENV === "1";
  const session = await login({ startedEnv });
  writeSession(session);
  print({
    ok: true,
    run_id: session.run_id,
    email: session.email,
    workspace_slug: session.workspace_slug,
    workspace_id: session.workspace_id,
    origin: session.origin,
    session_file: sessionPath(session.run_id),
    inject: {
      localStorage: {
        multica_token: session.token,
        "multica:chat:isOpen": "false",
        multica_create_mode: JSON.stringify({
          state: { lastMode: "manual" },
          version: 0,
        }),
      },
    },
  });
}

async function driveIssues(runId) {
  loadCheckoutEnv();
  const session = runId ? readSession(runId) : await login({
    startedEnv: process.env.VERIFY_STARTED_ENV === "1",
  });
  writeSession(session);

  let chromium;
  try {
    ({ chromium } = await import("@playwright/test"));
  } catch {
    fail("Cannot import @playwright/test. Run this from the repo root after pnpm install.");
  }

  const outDir = path.join(artifactsRoot, session.run_id, "issues");
  mkdirSync(outDir, { recursive: true });
  const title = `Verify issue ${session.run_id}`;
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    locale: "en-US",
    viewport: { width: 1440, height: 900 },
    baseURL: session.origin,
  });
  await context.addInitScript(
    ({ token, createMode }) => {
      localStorage.setItem("multica_token", token);
      localStorage.setItem("multica:chat:isOpen", "false");
      localStorage.setItem("multica_create_mode", createMode);
    },
    {
      token: session.token,
      createMode: JSON.stringify({ state: { lastMode: "manual" }, version: 0 }),
    },
  );

  const page = await context.newPage();
  const evidence = {
    feature: "issues",
    entry_point: "New Issue button on /:slug/issues",
    title,
    steps: [],
  };

  try {
    await page.goto(`/${session.workspace_slug}/issues`, {
      waitUntil: "domcontentloaded",
    });
    await page.getByRole("button", { name: "New Issue" }).waitFor({ timeout: 20000 });
    await page.screenshot({ path: path.join(outDir, "01-issues-board.png"), fullPage: true });
    evidence.steps.push({
      action: "open issues",
      url: page.url(),
      title: await page.title(),
      new_issue_visible: true,
    });

    await page.getByRole("button", { name: "New Issue" }).click();
    const titleInput = page.getByRole("textbox", { name: "Issue title" });
    await titleInput.waitFor({ timeout: 10000 });
    await page.screenshot({ path: path.join(outDir, "02-create-modal.png") });
    await titleInput.fill(title);
    await page.getByRole("button", { name: "Create Issue" }).click();
    await page.getByText("Issue created").waitFor({ timeout: 15000 });
    evidence.steps.push({ action: "create", toast: "Issue created" });

    await page.getByRole("button", { name: "View issue" }).click();
    await page.waitForURL(/\/issues\/[\w-]+/, { timeout: 15000 });
    await page.getByText("Properties").waitFor({ timeout: 15000 });
    const detailTitle = await page.title();
    if (!detailTitle.includes(title) || !detailTitle.endsWith("| Multica")) {
      fail("Issue detail tab title did not include the created title", { detailTitle });
    }
    await page.screenshot({ path: path.join(outDir, "03-issue-detail.png"), fullPage: true });
    const bodyText = await page.locator("body").innerText();
    writeFileSync(path.join(outDir, "03-issue-detail.txt"), bodyText);
    evidence.steps.push({
      action: "open detail",
      url: page.url(),
      title: detailTitle,
      properties_visible: bodyText.includes("Properties"),
      title_visible: bodyText.includes(title),
    });

    const created = await authedFetch(
      session.token,
      "/api/issues?limit=50",
      {},
      session.workspace_id,
      session.workspace_slug,
    );
    const issues = Array.isArray(created.body)
      ? created.body
      : Array.isArray(created.body?.issues)
        ? created.body.issues
        : [];
    const match = issues.find((item) => item.title === title);
    if (!match) {
      evidence.warning = "List endpoint shape did not include the new title; detail UI still showed it.";
    } else {
      session.created_issue_ids.push(match.id);
      writeSession(session);
      evidence.issue_id = match.id;
    }

    evidence.ok = true;
    evidence.artifacts = [
      path.join(outDir, "01-issues-board.png"),
      path.join(outDir, "02-create-modal.png"),
      path.join(outDir, "03-issue-detail.png"),
      path.join(outDir, "03-issue-detail.txt"),
    ];
    writeFileSync(path.join(outDir, "evidence.json"), `${JSON.stringify(evidence, null, 2)}\n`);
    print({
      ok: true,
      run_id: session.run_id,
      feature: "issues",
      evidence_dir: outDir,
      url: page.url(),
      title: detailTitle,
    });
  } finally {
    await browser.close();
  }
}

async function cleanup(runId, { stopEnv = false } = {}) {
  loadCheckoutEnv();
  const session = readSession(runId);
  const deleted = [];
  for (const id of session.created_issue_ids ?? []) {
    const res = await authedFetch(
      session.token,
      `/api/issues/${id}`,
      { method: "DELETE" },
      session.workspace_id,
      session.workspace_slug,
    );
    deleted.push({ issue_id: id, status: res.status });
  }

  if (session.workspace_slug?.startsWith("verify-") && session.workspace_id) {
    const res = await authedFetch(session.token, `/api/workspaces/${session.workspace_id}`, {
      method: "DELETE",
    });
    deleted.push({ workspace_id: session.workspace_id, status: res.status });
  }

  if (session.email?.startsWith("verify-")) {
    try {
      const { default: pg } = await import("pg");
      const client = new pg.Client(databaseUrl());
      await client.connect();
      try {
        await client.query(`DELETE FROM "user" WHERE email = $1`, [session.email]);
        deleted.push({ user_email: session.email, via: "db" });
      } finally {
        await client.end();
      }
    } catch (error) {
      deleted.push({ user_email: session.email, error: String(error) });
    }
  }

  let stoppedEnv = false;
  if (stopEnv && session.started_env === true) {
    execSync("make down", { cwd: repoRoot, stdio: "inherit" });
    stoppedEnv = true;
  }

  print({
    ok: true,
    run_id: runId,
    deleted,
    stopped_env: stoppedEnv,
    evidence_kept: path.join(artifactsRoot, runId),
  });
}

function usage() {
  print({
    usage: [
      "node .agents/skills/verify-multica/control-multica.mjs doctor",
      "node .agents/skills/verify-multica/control-multica.mjs login",
      "node .agents/skills/verify-multica/control-multica.mjs drive issues [--run-id <id>]",
      "node .agents/skills/verify-multica/control-multica.mjs cleanup --run-id <id> [--stop-env]",
    ],
  });
}

const args = process.argv.slice(2);
const cmd = args[0];
const flag = (name) => {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : undefined;
};
const has = (name) => args.includes(name);

if (cmd === "doctor") {
  await doctor();
} else if (cmd === "login") {
  await cmdLogin();
} else if (cmd === "drive" && args[1] === "issues") {
  await driveIssues(flag("--run-id"));
} else if (cmd === "cleanup") {
  const runId = flag("--run-id");
  if (!runId) fail("cleanup requires --run-id");
  await cleanup(runId, { stopEnv: has("--stop-env") });
} else {
  usage();
  process.exit(cmd ? 1 : 0);
}
