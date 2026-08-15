import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { expect, test, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const APP_URL = process.env.PLAYWRIGHT_BASE_URL
  ?? process.env.FRONTEND_ORIGIN
  ?? "http://localhost:3000";

interface BrowserSignals {
  consoleErrors: string[];
  pageErrors: string[];
  requestFailures: string[];
}

function collectSignals(page: Page): BrowserSignals {
  const signals: BrowserSignals = { consoleErrors: [], pageErrors: [], requestFailures: [] };
  page.on("console", (message) => {
    if (message.type() === "error") signals.consoleErrors.push(message.text());
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

async function refreshWiki(page: Page) {
  const [response] = await Promise.all([
    page.waitForResponse((candidate) => (
      candidate.url().endsWith("/api/lm-wiki/refresh")
      && candidate.request().method() === "POST"
    )),
    page.getByRole("button", { name: "Refresh Wiki" }).click(),
  ]);
  expect(response.ok()).toBe(true);
}

async function capture(page: Page, testInfo: TestInfo, label: string, width: number, height: number) {
  await page.setViewportSize({ width, height });
  await expect(page.locator("[data-twin-workspace]")).toBeVisible();
  const dimensions = await page.evaluate(() => ({
    viewport: window.innerWidth,
    documentWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(dimensions.documentWidth).toBeLessThanOrEqual(dimensions.clientWidth);
  const outputDir = process.env.LM_WIKI_POLISH_EVIDENCE_DIR ?? testInfo.outputDir;
  await mkdir(outputDir, { recursive: true });
  await page.screenshot({
    path: path.join(outputDir, `${label}-${width}x${height}.png`),
    animations: "disabled",
  });
  return { label, ...dimensions };
}

test("keeps Wiki and Twin edge states actionable, accurate, and responsive", async ({ page }, testInfo) => {
  test.setTimeout(300_000);
  const commitSha = process.env.LM_WIKI_POLISH_COMMIT_SHA;
  if (!commitSha) throw new Error("LM_WIKI_POLISH_COMMIT_SHA is required for evidence provenance");
  const runId = `${Date.now().toString(36)}-${process.pid.toString(36)}`;
  const email = `twin-polish-${runId}@multica.ai`;
  const workspaceSlug = `twin-polish-${runId}`;
  const owner = new TestApiClient();
  const signals = collectSignals(page);
  const screenshots: Array<Awaited<ReturnType<typeof capture>>> = [];
  let workspace: { id: string; slug: string } | null = null;

  let testFailure: unknown;
  let testFailed = false;
  try {
    await owner.login(email, "Twin Polish Owner");
    workspace = await owner.ensureWorkspace(`Twin Polish ${runId}`, workspaceSlug);
    await owner.markUserOnboarded();
    const token = owner.getToken();
    if (!token) throw new Error("Owner login did not return a token");
    await authenticate(page, token);
    await page.route("**/api/lm-wiki/", async (route) => {
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          latest_revision: null,
          accepted_revision: null,
          pending_revision: null,
          revisions: [],
          can_manage: true,
        }),
      });
    });
    await page.route("**/api/twins/", async (route) => {
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          current_version: null,
          pending_proposal: null,
          proposals: [],
          versions: [],
          can_manage: true,
        }),
      });
    });
    await page.route("**/api/twin/overview", async (route) => {
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({ twin: null }),
      });
    });
    await page.goto(`/${workspace.slug}/twins`, { waitUntil: "domcontentloaded" });
    await expect(page.getByText("Refresh Wiki to compile the first evidence revision.")).toBeVisible({
      timeout: 120_000,
    });

    for (const [width, height] of [[1280, 900], [768, 1024], [375, 844]] as const) {
      screenshots.push(await capture(page, testInfo, "empty-wiki", width, height));
    }
    await page.getByRole("tab", { name: "Twin Builder" }).click();
    await expect(page.getByText("Accept a Wiki revision to start Twin Builder.")).toBeVisible();
    await expect(page.getByRole("combobox", { name: "Twin proposal" })).toHaveCount(0);
    await expect(page.getByRole("combobox", { name: "Twin version" })).toHaveCount(0);
    for (const [width, height] of [[1280, 900], [768, 1024], [375, 844]] as const) {
      screenshots.push(await capture(page, testInfo, "empty-twin", width, height));
    }
    await page.emulateMedia({ reducedMotion: "reduce" });
    const localeCases = [
      ["zh-Hans", "zh-CN", "Twin 构建器", "接受一个 Wiki 修订后即可开始构建 Twin。", "zh"],
      ["ja", "ja-JP", "Twin Builder", "Wiki リビジョンを承認すると Twin\u00a0Builder を開始できます。", "ja"],
      ["ko", "ko-KR", "Twin Builder", "Wiki 리비전을 승인한 뒤 Twin\u00a0Builder를 시작하세요.", "ko"],
    ] as const;
    for (const [locale, lang, tab, copy, label] of localeCases) {
      await page.context().addCookies([{ name: "multica-locale", value: locale, url: APP_URL }]);
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(page.locator("html")).toHaveAttribute("lang", lang);
      await page.getByRole("tab", { name: tab }).click();
      await expect(page.getByText(copy)).toBeVisible();
      screenshots.push(await capture(page, testInfo, `${label}-empty-twin-reduced-motion`, 375, 844));
    }

    await page.context().addCookies([{ name: "multica-locale", value: "en", url: APP_URL }]);
    await page.emulateMedia({ reducedMotion: "no-preference" });
    await page.unroute("**/api/lm-wiki/");
    await page.unroute("**/api/twins/");
    await page.unroute("**/api/twin/overview");
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.locator("html")).toHaveAttribute("lang", "en");

    await owner.createIssue(`Edge source ${"X".repeat(120)}`, {
      description: `Long evidence ${"Y".repeat(600)}`,
      status: "in_review",
      priority: "high",
    });
    await page.getByRole("tab", { name: "LM Wiki" }).click();
    await refreshWiki(page);
    let wiki = await owner.getLMWiki();
    if (!wiki.latest_revision) throw new Error("Refresh did not produce a Wiki revision");
    await expect(page.getByRole("heading", {
      name: `Revision r${wiki.latest_revision.revision_number}`,
    })).toBeVisible();
    await page.getByRole("button", { name: "Reject revision" }).click();
    const reason = page.getByLabel("Reason");
    await expect(reason).toHaveAttribute("maxlength", "2000");
    await reason.fill("R".repeat(2000));
    await expect(page.getByText("2000 / 2000 characters")).toBeVisible();
    for (const [width, height] of [[1280, 900], [768, 1024], [375, 844]] as const) {
      screenshots.push(await capture(page, testInfo, "reason-limit", width, height));
    }
    await page.getByRole("button", { name: "Cancel" }).click();

    let releaseAccept = () => undefined;
    const acceptGate = new Promise<void>((resolve) => {
      releaseAccept = resolve;
    });
    await page.route("**/api/lm-wiki/revisions/*/accept", async (route) => {
      await acceptGate;
      await route.continue();
    }, { times: 1 });
    const acceptResponse = page.waitForResponse((response) => response.url().includes("/accept"));
    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(page.getByRole("button", { name: "Saving decision" })).toBeDisabled();
    await page.getByRole("button", { name: "Dismiss" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    releaseAccept();
    expect((await acceptResponse).ok()).toBe(true);

    await page.getByRole("tab", { name: "Twin Builder" }).click();
    await page.getByRole("button", { name: "Sign off proposal" }).click();
    await page.getByRole("button", { name: "Confirm sign-off" }).click();
    await expect(page.getByRole("heading", { name: "Current Twin v1" })).toBeVisible();

    await owner.createIssue("Second generation evidence", { status: "todo" });
    await page.getByRole("tab", { name: "LM Wiki" }).click();
    await refreshWiki(page);
    wiki = await owner.getLMWiki();
    if (!wiki.latest_revision) throw new Error("Second refresh did not produce a Wiki revision");
    await expect(page.getByRole("heading", {
      name: `Revision r${wiki.latest_revision.revision_number}`,
    })).toBeVisible();
    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await page.getByRole("tab", { name: "Twin Builder" }).click();
    await page.getByRole("button", { name: "Sign off proposal" }).click();
    await page.getByRole("button", { name: "Confirm sign-off" }).click();
    await expect(page.getByRole("heading", { name: "Current Twin v2" })).toBeVisible();

    await page.getByRole("combobox", { name: "Twin version" }).click();
    await page.getByRole("option", { name: /^v1 \/ / }).click();
    await expect(page.getByRole("heading", { name: "Current Twin v2" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Version v1" })).toBeVisible();
    for (const [width, height] of [[1280, 900], [768, 1024], [375, 844]] as const) {
      screenshots.push(await capture(page, testInfo, "historical-version", width, height));
    }

    await page.getByRole("tab", { name: "LM Wiki" }).click();
    await page.getByRole("combobox", { name: "Wiki revision" }).click();
    const oldestRevision = page.getByRole("option").filter({ hasText: /^r\d+ \/ / }).last();
    const oldestRevisionLabel = await oldestRevision.textContent();
    if (!oldestRevisionLabel) throw new Error("Wiki history did not expose an older revision");
    await oldestRevision.click();
    const oldestRevisionNumber = oldestRevisionLabel.split(" ", 1)[0]?.slice(1);
    if (!oldestRevisionNumber) throw new Error("Wiki revision label did not include a number");
    await expect(page.getByRole("heading", { name: `Revision r${oldestRevisionNumber}` })).toBeVisible();
    await owner.createIssue("Third generation evidence", { status: "in_progress" });
    await refreshWiki(page);
    wiki = await owner.getLMWiki();
    if (!wiki.latest_revision) throw new Error("Third refresh did not produce a Wiki revision");
    await expect(page.getByRole("heading", {
      name: `Revision r${wiki.latest_revision.revision_number}`,
    })).toBeVisible();
    screenshots.push(await capture(page, testInfo, "refresh-follows-result", 375, 844));

    expect(signals).toEqual({ consoleErrors: [], pageErrors: [], requestFailures: [] });
    const outputDir = process.env.LM_WIKI_POLISH_EVIDENCE_DIR ?? testInfo.outputDir;
    await writeFile(
      path.join(outputDir, "browser-actions.json"),
      JSON.stringify({ commitSha, screenshots, signals }, null, 2),
    );
  } catch (error) {
    testFailure = error;
    testFailed = true;
  } finally {
    const cleanupFailures: unknown[] = [];
    const attemptCleanup = async (cleanup: () => Promise<void>) => {
      try {
        await cleanup();
      } catch (error) {
        cleanupFailures.push(error);
      }
    };

    await attemptCleanup(() => owner.cleanup());
    if (workspace) {
      await attemptCleanup(() => owner.deleteWorkspace(workspace.id));
      await attemptCleanup(async () => {
        expect(await owner.getWikiTwinArtifactCounts(workspace.id)).toEqual({
          wiki_revisions: 0,
          wiki_citations: 0,
          wiki_reviews: 0,
          twin_proposals: 0,
          twin_reviews: 0,
          twin_versions: 0,
        });
      });
    }
    await attemptCleanup(() => owner.deleteUser());

    if (testFailed) throw testFailure;
    if (cleanupFailures.length > 0) throw cleanupFailures[0];
  }
});
