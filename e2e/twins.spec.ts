import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { expect, test, type BrowserContext, type Page, type TestInfo } from "@playwright/test";
import pg from "pg";
import { TestApiClient, type TestWikiTwinArtifactCounts } from "./fixtures";

const DATABASE_URL = process.env.DATABASE_URL
  ?? "postgres://multica:multica@localhost:5432/multica?sslmode=disable";
const APP_URL = process.env.PLAYWRIGHT_BASE_URL ?? "http://localhost:3000";

interface BrowserSignals {
  consoleErrors: string[];
  expectedConflicts: string[];
  pageErrors: string[];
  requestFailures: string[];
}

function collectSignals(page: Page): BrowserSignals {
  const signals: BrowserSignals = {
    consoleErrors: [],
    expectedConflicts: [],
    pageErrors: [],
    requestFailures: [],
  };
  page.on("console", (message) => {
    if (message.type() !== "error") return;
    const text = message.text();
    const structuredStaleConflict = text.includes("409")
      && text.includes("/api/lm-wiki/revisions/")
      && text.includes("/accept")
      && text.includes("LM Wiki revision is stale");
    if (text.includes("409 (Conflict)") || structuredStaleConflict) {
      signals.expectedConflicts.push(text);
    }
    else signals.consoleErrors.push(text);
  });
  page.on("pageerror", (error) => signals.pageErrors.push(error.message));
  page.on("requestfailed", (request) => {
    const failure = request.failure()?.errorText ?? "unknown failure";
    if (request.url().includes("/api/client-usage") && failure.includes("ERR_ABORTED")) return;
    signals.requestFailures.push(`${request.method()} ${request.url()} ${failure}`);
  });
  return signals;
}

async function authenticate(page: Page, token: string) {
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
}

async function capture(
  page: Page,
  testInfo: TestInfo,
  label: string,
  width: number,
  height: number,
) {
  await page.setViewportSize({ width, height });
  await expect(page.locator("[data-twin-workspace]")).toBeVisible();
  const overflow = await page.evaluate(() => ({
    viewport: window.innerWidth,
    documentWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(overflow.documentWidth).toBeLessThanOrEqual(overflow.clientWidth);
  const outputDir = process.env.LM_WIKI_EVIDENCE_DIR ?? testInfo.outputDir;
  await mkdir(outputDir, { recursive: true });
  await page.screenshot({
    path: path.join(outputDir, `${label}-${width}x${height}.png`),
    animations: "disabled",
  });
  return overflow;
}

async function addWorkspaceMember(email: string, workspaceId: string) {
  const client = new pg.Client(DATABASE_URL);
  await client.connect();
  try {
    await client.query(
      `
        INSERT INTO member (workspace_id, user_id, role)
        SELECT $1::uuid, id, 'member' FROM "user" WHERE email = $2
        ON CONFLICT (workspace_id, user_id) DO UPDATE SET role = 'member'
      `,
      [workspaceId, email],
    );
  } finally {
    await client.end();
  }
}

async function deleteTestUsers(emails: readonly string[]) {
  const client = new pg.Client(DATABASE_URL);
  await client.connect();
  try {
    await client.query(`DELETE FROM "user" WHERE email = ANY($1::text[])`, [emails]);
  } finally {
    await client.end();
  }
}

function zeroCounts(): TestWikiTwinArtifactCounts {
  return {
    wiki_revisions: 0,
    wiki_citations: 0,
    wiki_reviews: 0,
    twin_proposals: 0,
    twin_reviews: 0,
    twin_versions: 0,
  };
}

test("reviews a live Wiki, signs initial and evolved Twins, and preserves member read-only access", async ({
  browser,
  page,
}, testInfo) => {
  test.setTimeout(300_000);
  const runId = `${Date.now().toString(36)}-${process.pid.toString(36)}-${testInfo.workerIndex}`;
  const ownerEmail = `twin-owner-${runId}@multica.ai`;
  const memberEmail = `twin-member-${runId}@multica.ai`;
  const workspaceSlug = `twin-e2e-${runId}`;
  const owner = new TestApiClient();
  const member = new TestApiClient();
  const actions: string[] = [];
  const ownerSignals = collectSignals(page);
  let memberContext: BrowserContext | null = null;
  let visualPage: Page | null = null;
  let visualSignals: BrowserSignals | null = null;
  let workspace: { id: string; slug: string } | null = null;
  let cleanupCounts: TestWikiTwinArtifactCounts | null = null;

  try {
    await owner.login(ownerEmail, "Twin E2E Owner");
    workspace = await owner.ensureWorkspace(`Twin E2E ${runId}`, workspaceSlug);
    await owner.markUserOnboarded();
    await owner.createIssue("Review the live Wiki source", {
      description: "A deterministic issue source for the Twin lifecycle.",
      status: "todo",
      priority: "high",
    });
    const ownerToken = owner.getToken();
    if (!ownerToken) throw new Error("Owner login did not return a token");
    await authenticate(page, ownerToken);
    await page.goto(`/${workspace.slug}/twins`, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "LM Wiki and Twin", exact: true })).toBeVisible({
      timeout: 120_000,
    });

    const firstRun = page.getByText(/first (?:Wiki refresh|evidence revision)/i);
    const refreshButton = page.getByRole("button", { name: "Refresh Wiki" });
    await expect(refreshButton).toBeVisible({ timeout: 30_000 });
    await capture(page, testInfo, "initial-wiki", 1280, 900);
    if (await firstRun.isVisible()) {
      await refreshButton.click();
      actions.push("manager explicitly created the first Wiki revision");
    } else {
      actions.push("daily reconciliation created the first Wiki revision");
    }
    await expect(page.getByText("Pending review").first()).toBeVisible();
    await expect(page.getByRole("heading", { name: "Revision r1" })).toBeVisible();
    actions.push("first Wiki revision pending");

    const citation = page.getByRole("button", { name: "Show citation" }).first();
    await citation.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByText(/Issue #1: Review the live Wiki source/)).toBeVisible();
    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await page.getByRole("tab", { name: "Twin Builder" }).click();
    await expect(page.getByRole("button", { name: "Sign off proposal" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Open issue" })).toBeVisible();
    await page.getByRole("button", { name: "Sign off proposal" }).click();
	    await page.getByRole("button", { name: "Confirm sign-off" }).click();
	    await expect(page.getByRole("heading", { name: "Current Twin v1" })).toBeVisible();
	    const initialReviewSpine = page.getByRole("region", { name: "Twin review progress" });
	    await expect(initialReviewSpine).toBeVisible();
	    await expect(initialReviewSpine.getByTestId("twin-review-step")).toHaveCount(6);
    await expect(initialReviewSpine.getByText("Deposition").locator("..")).toHaveAttribute("data-state", "current");
	    actions.push("initial Twin v1 signed");

    await page.reload({ waitUntil: "domcontentloaded" });
    await page.getByRole("tab", { name: "Twin Builder" }).click();
    await expect(page.getByRole("heading", { name: "Current Twin v1" })).toBeVisible();
    await owner.createIssue("Evolve the Twin evidence", { status: "in_progress" });
    await page.getByRole("tab", { name: "LM Wiki" }).click();
    await page.getByRole("button", { name: "Refresh Wiki" }).click();
    await expect(page.getByRole("heading", { name: "Revision r2" })).toBeVisible();

    await owner.createIssue("Make the visible review stale", { status: "in_review" });
    const latestRefresh = await owner.refreshLMWiki();
    expect(latestRefresh.revision.revision_number).toBe(3);
    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(page.getByTestId("twin-workspace-content").getByRole("alert"))
      .toHaveText("This review is out of date. Check the latest version and try again.");
    await expect(page.getByRole("heading", { name: "Revision r3" })).toBeVisible();
    actions.push("stale Wiki review surfaced and canonical revision reloaded");

    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await page.getByRole("tab", { name: "Twin Builder" }).click();
    const selectedProposal = page.locator("section").filter({
      has: page.getByRole("heading", { name: "Selected proposal" }),
    });
    await expect(selectedProposal.getByText(/^Added assertions/)).toBeVisible();
    await page.getByRole("button", { name: "Reject proposal" }).click();
    await page.getByLabel("Reason").fill("Evidence needs another revision");
    await page.getByRole("button", { name: "Confirm rejection" }).click();
    await expect(page.getByRole("button", { name: "Sign off proposal" })).toHaveCount(0);
    await expect(page.getByText("rejected")).toBeVisible();
    await expect(page.getByRole("button", { name: "Build proposal" })).toHaveCount(0);
    await owner.createIssue("Continue Twin evolution", { status: "todo" });
    await page.getByRole("tab", { name: "LM Wiki" }).click();
    await page.getByRole("button", { name: "Refresh Wiki" }).click();
    await expect(page.getByRole("heading", { name: "Revision r4" })).toBeVisible();
    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await page.getByRole("tab", { name: "Twin Builder" }).click();
    await expect(page.getByRole("button", { name: "Sign off proposal" })).toBeVisible();
    await page.getByRole("button", { name: "Sign off proposal" }).click();
	    await page.getByRole("button", { name: "Confirm sign-off" }).click();
	    await expect(page.getByRole("heading", { name: "Current Twin v2" })).toBeVisible();
	    const evolvedReviewSpine = page.getByRole("region", { name: "Twin review progress" });
	    await expect(evolvedReviewSpine.getByTestId("twin-review-step")).toHaveCount(6);
    await expect(evolvedReviewSpine.getByText("Deposition").locator("..")).toHaveAttribute("data-state", "complete");
	    actions.push("rejected evolution preserved; changed sources produced and signed Twin v2");

    visualPage = await page.context().newPage();
    await authenticate(visualPage, ownerToken);
    visualSignals = collectSignals(visualPage);
    await visualPage.goto(`/${workspace.slug}/twins`, { waitUntil: "domcontentloaded" });
    await expect(visualPage.getByRole("heading", { name: "LM Wiki and Twin", exact: true })).toBeVisible({
      timeout: 30_000,
    });
    await visualPage.getByRole("tab", { name: "Twin Builder" }).click();
    await expect(visualPage.getByRole("heading", { name: "Current Twin v2" })).toBeVisible();
    const responsive = [];
    responsive.push(await capture(visualPage, testInfo, "signed-twin", 1280, 900));
    responsive.push(await capture(visualPage, testInfo, "signed-twin", 768, 1024));
    responsive.push(await capture(visualPage, testInfo, "signed-twin", 375, 844));
    const sidebarTrigger = visualPage.getByRole("button", { name: "Toggle Sidebar" });
    await sidebarTrigger.click();
    await expect(visualPage.getByRole("dialog", { name: "Sidebar" })).toBeVisible();
    responsive.push(await capture(visualPage, testInfo, "signed-twin-sidebar", 375, 844));
    await visualPage.keyboard.press("Escape");
    await expect(visualPage.getByRole("dialog", { name: "Sidebar" })).toHaveCount(0);
    await expect(sidebarTrigger).toBeFocused();
    await visualPage.getByRole("tab", { name: "LM Wiki" }).click();
    responsive.push(await capture(visualPage, testInfo, "accepted-wiki", 1280, 900));
    responsive.push(await capture(visualPage, testInfo, "accepted-wiki", 768, 1024));
    responsive.push(await capture(visualPage, testInfo, "accepted-wiki", 375, 844));
    await visualPage.context().addCookies([{ name: "multica-locale", value: "zh-Hans", url: APP_URL }]);
    await visualPage.emulateMedia({ reducedMotion: "reduce" });
    await visualPage.reload({ waitUntil: "domcontentloaded" });
    await expect(visualPage.locator("html")).toHaveAttribute("lang", "zh-CN");
    await capture(visualPage, testInfo, "zh-reduced-motion", 375, 844);
    expect(visualSignals.consoleErrors).toEqual([]);
    expect(visualSignals.pageErrors).toEqual([]);
    expect(visualSignals.requestFailures).toEqual([]);
    await visualPage.close();
    visualPage = null;

    await member.login(memberEmail, "Twin E2E Member");
    await member.markUserOnboarded();
    await addWorkspaceMember(memberEmail, workspace.id);
    member.setWorkspaceId(workspace.id);
    member.setWorkspaceSlug(workspace.slug);
    const memberToken = member.getToken();
    if (!memberToken) throw new Error("Member login did not return a token");
    memberContext = await browser.newContext({ viewport: { width: 768, height: 1024 } });
    await memberContext.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
    }, memberToken);
    const memberPage = await memberContext.newPage();
    const memberSignals = collectSignals(memberPage);
    await memberPage.goto(`/${workspace.slug}/twins`, { waitUntil: "domcontentloaded" });
    await expect(memberPage.getByText("Read-only access")).toBeVisible({ timeout: 30_000 });
    await expect(memberPage.getByRole("button", { name: "Refresh Wiki" })).toHaveCount(0);
    await memberPage.getByRole("tab", { name: "Twin Builder" }).click();
    await expect(memberPage.getByRole("heading", { name: "Current Twin v2" })).toBeVisible();
    await expect(memberPage.getByRole("button", { name: "Sign off proposal" })).toHaveCount(0);
    const denied = await member.request("/api/lm-wiki/refresh", { method: "POST" });
    expect(denied.status).toBe(403);
    expect(memberSignals.consoleErrors).toEqual([]);
    expect(memberSignals.pageErrors).toEqual([]);
    expect(memberSignals.requestFailures).toEqual([]);
    actions.push("member read-only UI and direct 403 verified");
    await memberContext.close();
    memberContext = null;

    expect(ownerSignals.consoleErrors).toEqual([]);
    expect(ownerSignals.pageErrors).toEqual([]);
    expect(ownerSignals.requestFailures).toEqual([]);
    expect(ownerSignals.expectedConflicts).toHaveLength(2);
    const outputDir = process.env.LM_WIKI_EVIDENCE_DIR ?? testInfo.outputDir;
    await writeFile(
      path.join(outputDir, "browser-actions.json"),
      JSON.stringify({ actions, responsive, ownerSignals, visualSignals }, null, 2),
    );
  } finally {
    if (visualPage) await visualPage.close();
    if (memberContext) await memberContext.close();
    await page.close();
    await owner.cleanup();
    if (workspace) {
      await owner.deleteWorkspace(workspace.id);
      cleanupCounts = await owner.getWikiTwinArtifactCounts(workspace.id);
      expect(cleanupCounts).toEqual(zeroCounts());
    }
    await deleteTestUsers([ownerEmail, memberEmail]);
  }
});
