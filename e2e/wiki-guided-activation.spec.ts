import { mkdir } from "node:fs/promises";
import path from "node:path";
import { expect, test, type Browser, type Page } from "@playwright/test";
import { TestApiClient, type TestWikiPage } from "./fixtures";

const evidenceDir = path.resolve("test-results/change-evidence/wiki-guided-activation");
const rawVideoDir = process.env.WIKI_EVIDENCE_VIDEO_DIR || evidenceDir;

test.describe.configure({ mode: "serial" });

test("records the pre-change immutable Wiki revision review", async ({ browser }) => {
  await recordWikiScenario(browser, {
    label: "baseline",
    artifact: "baseline-immutable-revision.webm",
    viewport: { width: 1280, height: 800 },
    run: async ({ browserPage }) => {
      const revisionView = browserPage.getByTestId("wiki-revision-page");
      await expect(revisionView).toBeVisible({ timeout: 30_000 });
      await expect(revisionView.locator("header").getByRole("heading", { name: "Release evidence policy" })).toBeVisible();
      const activation = revisionView.getByTestId("wiki-knowledge-activation");
      await activation.evaluate((element) => { element.setAttribute("hidden", "true"); });
      await expect(activation).toBeHidden();
      await browserPage.waitForTimeout(2_000);
      await revisionView.getByText("Production releases require an owner-reviewed rollback plan.")
        .scrollIntoViewIfNeeded();
      await browserPage.waitForTimeout(2_000);
    },
  });
});

test("records desktop exact-revision activation acceptance", async ({ browser }) => {
  await recordWikiScenario(browser, {
    label: "desktop",
    artifact: "accepted-desktop-exact-revision.webm",
    viewport: { width: 1280, height: 800 },
    run: async ({ api, browserPage, wikiPage }) => {
      const revisionView = browserPage.getByTestId("wiki-revision-page");
      await expect(revisionView).toBeVisible({ timeout: 30_000 });
      await expect(revisionView.getByText("Eligible, not pinned")).toBeVisible({ timeout: 30_000 });
      const activation = revisionView.getByRole("button", { name: "Use as LM Wiki evidence" });
      await expect(activation).toBeVisible();
      await browserPage.waitForTimeout(1_500);
      await activation.click();

      const dialog = browserPage.getByRole("dialog");
      await expect(dialog).toContainText("Use this exact revision as LM Wiki evidence?");
      await expect(dialog).toContainText("operations/release-evidence.md");
      await expect(dialog).toContainText("sha256:");
      await expect(dialog).toContainText("Remote generation is disabled");
      await expect(dialog).toContainText("Personal Wiki pages");
      await browserPage.waitForTimeout(2_500);
      await dialog.getByRole("button", { name: "Pin exact revision" }).click();

      await expect(revisionView.getByRole("button", { name: "Exact revision pinned" }))
        .toBeVisible({ timeout: 30_000 });
      await expect(revisionView.getByText("LM Wiki refresh required")).toBeVisible();
      const policy = await api.getLMWikiSourcePolicy();
      expect(policy.wiki_pages).toContainEqual({ page_id: wikiPage.id, revision_number: 1 });
      await browserPage.waitForTimeout(2_500);

      await api.updateWikiPage(wikiPage.id, {
        expected_revision_number: 1,
        content: "# Release evidence policy\n\nProduction releases also require a recovery drill owner.",
      });
      await browserPage.reload({ waitUntil: "domcontentloaded" });
      await expect(browserPage.getByTestId("wiki-revision-page").getByText("Newer revision available"))
        .toBeVisible({ timeout: 30_000 });
      await browserPage.waitForTimeout(2_500);
    },
  });
});

test("records narrow-viewport exact-revision confirmation", async ({ browser }) => {
  await recordWikiScenario(browser, {
    label: "narrow",
    artifact: "accepted-narrow-exact-revision.webm",
    viewport: { width: 390, height: 844 },
    run: async ({ browserPage }) => {
      const revisionView = browserPage.getByTestId("wiki-revision-page");
      await expect(revisionView).toBeVisible({ timeout: 30_000 });
      const activation = revisionView.getByRole("button", { name: "Use as LM Wiki evidence" });
      await activation.scrollIntoViewIfNeeded();
      await activation.click();
      const dialog = browserPage.getByRole("dialog");
      await expect(dialog).toBeVisible();
      await expect(dialog).toContainText("Exact revision");
      await expect(dialog).toContainText("Current source policy");
      await expect(dialog).toContainText("Remote generation is disabled");
      await browserPage.waitForTimeout(2_500);
      const confirm = dialog.getByRole("button", { name: "Pin exact revision" });
      await confirm.scrollIntoViewIfNeeded();
      await confirm.click();
      await expect(revisionView.getByRole("button", { name: "Exact revision pinned" }))
        .toBeVisible({ timeout: 30_000 });
      await browserPage.waitForTimeout(2_500);
    },
  });
});

interface WikiScenarioOptions {
  label: string;
  artifact: string;
  viewport: { width: number; height: number };
  beforeNavigation?: (browserPage: Page) => Promise<void>;
  run: (fixture: {
    api: TestApiClient;
    browserPage: Page;
    wikiPage: TestWikiPage;
  }) => Promise<void>;
}

async function recordWikiScenario(browser: Browser, options: WikiScenarioOptions) {
  await Promise.all([
    mkdir(evidenceDir, { recursive: true }),
    mkdir(rawVideoDir, { recursive: true }),
  ]);
  const runId = `${Date.now().toString(36)}-${process.pid.toString(36)}`;
  const api = new TestApiClient();
  let workspace: { id: string; slug: string } | null = null;
  let pageId = "";
  const context = await browser.newContext({
    viewport: options.viewport,
    recordVideo: { dir: rawVideoDir, size: options.viewport },
  });
  const browserPage = await context.newPage();
  const video = browserPage.video();

  try {
    await api.login(`wiki-activation-${options.label}-${runId}@multica.ai`, `Wiki Activation ${options.label}`);
    workspace = await api.ensureWorkspace(
      `Wiki Activation ${options.label} ${runId}`,
      `wiki-activation-${options.label}-${runId}`,
    );
    await api.markUserOnboarded();
    const wikiPage = await api.createWikiPage({
      scope: "workspace",
      path: "operations/release-evidence.md",
      title: "Release evidence policy",
      content: [
        "# Release evidence policy",
        "",
        "Production releases require an owner-reviewed rollback plan.",
      ].join("\n"),
    });
    pageId = wikiPage.id;
    const token = api.getToken();
    if (!token) throw new Error("evidence login did not return a token");
    await browserPage.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
    }, token);
    await options.beforeNavigation?.(browserPage);
    await browserPage.goto(
      `/${workspace.slug}/wiki/revisions/${wikiPage.current_revision_id}`,
      { waitUntil: "domcontentloaded", timeout: 30_000 },
    );
    await options.run({ api, browserPage, wikiPage });
  } finally {
    try {
      await context.close();
      if (video) await video.saveAs(path.join(evidenceDir, options.artifact));
    } finally {
      if (pageId) await api.deleteWikiPage(pageId).catch(() => undefined);
      if (workspace) await api.deleteWorkspace(workspace.id).catch(() => undefined);
      await api.deleteUser().catch(() => undefined);
    }
  }
}
