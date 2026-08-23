import { createHash } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { expect, test, type BrowserContext, type Page, type TestInfo } from "@playwright/test";
import pg from "pg";
import {
  TestApiClient,
  type TestLMWikiSourcePolicyInput,
  type TestWikiTwinArtifactCounts,
} from "./fixtures";

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
    // Navigating from the live Workspace Wiki to an issue or immutable
    // revision cancels its in-flight list refresh. This is a browser-owned
    // cancellation, not a backend response failure.
    if (request.method() === "GET" && failure.includes("ERR_ABORTED")) {
      const pathname = new URL(request.url()).pathname;
      if (pathname === "/api/wiki/pages" || pathname.startsWith("/api/wiki/pages/")) return;
    }
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
  rootSelector = "[data-twin-workspace]",
) {
  await page.setViewportSize({ width, height });
  await expect(page.locator(rootSelector)).toBeVisible();
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
    workspace_wiki_pages: 0,
    workspace_wiki_revisions: 0,
    workspace_wiki_proposals: 0,
    wiki_source_policies: 0,
    wiki_source_selections: 0,
    wiki_revisions: 0,
    wiki_citations: 0,
    wiki_reviews: 0,
    twin_proposals: 0,
    twin_reviews: 0,
    twin_versions: 0,
  };
}

test("reviews Workspace Wiki evidence, signs evolved Twins, and preserves member read-only access", async ({
  browser,
  page,
}, testInfo) => {
  test.setTimeout(900_000);
  page.setDefaultTimeout(30_000);
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
  let personalKnowledgePage: Page | null = null;
  let visualSignals: BrowserSignals | null = null;
  let workspace: { id: string; slug: string } | null = null;
  let personalPageId: string | null = null;
  let cleanupCounts: TestWikiTwinArtifactCounts | null = null;

  try {
    await owner.login(ownerEmail, "Twin E2E Owner");
    workspace = await owner.ensureWorkspace(`Twin E2E ${runId}`, workspaceSlug);
    await owner.markUserOnboarded();
    const sourceIssue = await owner.createIssue("Review the live Wiki source", {
      description: "A deterministic issue source for the Twin lifecycle.",
      status: "todo",
      priority: "high",
    });
    const sharedPage = await owner.createWikiPage({
      scope: "workspace",
      path: "operations/release-runbook.md",
      title: "Release runbook",
      content: "# Release runbook\n\nStart with the signed release checklist.\n",
    });
    expect(sharedPage.current_revision_number).toBe(1);
    const personalPage = await owner.createWikiPage({
      scope: "user",
      path: "private/release-notes.md",
      title: "Private release notes",
      content: "# Private notes\n\nNever leave the personal knowledge boundary.\n",
    });
    personalPageId = personalPage.id;

    const humanRevision = await owner.updateWikiPage(sharedPage.id, {
      expected_revision_number: 1,
      content: "# Release runbook\n\nStart with the signed checklist and verify rollback ownership.\n",
    });
    expect(humanRevision.current_revision_number).toBe(2);
    const staleUpdate = await owner.request(`/api/wiki/pages/${sharedPage.id}/`, {
      method: "PUT",
      body: JSON.stringify({
        expected_revision_number: 1,
        content: "# Stale overwrite\n",
      }),
    });
    expect(staleUpdate.status).toBe(409);
    await expect(staleUpdate.json()).resolves.toMatchObject({
      code: "wiki_revision_conflict",
      current_revision_number: 2,
    });
    const searchResults = await owner.searchWikiPages("rollback ownership", "workspace");
    expect(searchResults.map((result) => result.id)).toContain(sharedPage.id);

    const agentCredential = await owner.createWikiAgentCredential(sourceIssue.id);
    const directAgentWrite = await owner.requestWithTaskToken(
      `/api/wiki/pages/${sharedPage.id}/`,
      agentCredential.taskToken,
      {
        method: "PUT",
        body: JSON.stringify({
          expected_revision_number: 2,
          content: "# Unreviewed Agent overwrite\n",
        }),
      },
    );
    expect(directAgentWrite.status).toBe(403);
    const proposalResponse = await owner.requestWithTaskToken(
      `/api/wiki/pages/${sharedPage.id}/proposals`,
      agentCredential.taskToken,
      {
        method: "POST",
        body: JSON.stringify({
          base_revision_number: 2,
          proposed_content: "# Release runbook\n\nAgent draft: verify rollback ownership and notify support.\n",
          rationale: "The issue evidence adds a support handoff.",
          evidence_refs: [`task:${agentCredential.taskId}`],
          agent_id: agentCredential.agentId,
          idempotency_key: `wiki-proposal-${runId}`,
        }),
      },
    );
    expect(proposalResponse.status).toBe(201);
    const proposal = await proposalResponse.json() as {
      id: string;
      status: string;
      base_revision_number: number;
    };
    expect(proposal).toMatchObject({ status: "pending", base_revision_number: 2 });
    const reviewedContent = [
      "# Release runbook",
      "",
      "Verify rollback ownership, notify support, and record the decision.",
      `[Open the source issue](/issues/${sourceIssue.identifier})`,
      "",
    ].join("\n");
    const ownerToken = owner.getToken();
    if (!ownerToken) throw new Error("Owner login did not return a token");
    await authenticate(page, ownerToken);
    await page.goto(`/${workspace.slug}/wiki/${sharedPage.id}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "Wiki", exact: true })).toBeVisible({
      timeout: 120_000,
    });
    await expect(page.getByTestId("wiki-page").getByRole(
      "heading",
      { name: "Release runbook", exact: true, level: 2 },
    )).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText("Revision 2", { exact: true })).toBeVisible();
    await page.getByRole("textbox", { name: "Search Wiki" }).fill("rollback ownership");
    await expect(page.locator("aside").getByText("Release runbook", { exact: true })).toBeVisible();
    await capture(page, testInfo, "workspace-wiki-search", 1280, 900, "[data-testid=wiki-page]");

    await page.getByRole("tab", { name: "Proposals" }).click();
    const proposalReview = page.getByRole("region", { name: "Review proposed edit" });
    await expect(proposalReview).toBeVisible();
    await page.getByRole("textbox", { name: "Content" }).fill(reviewedContent);
    await proposalReview.getByText("Preview", { exact: true }).click();
    await expect(proposalReview.getByRole("link", { name: "Open the source issue" }))
      .toHaveAttribute("href", `/${workspace.slug}/issues/${sourceIssue.identifier}`);
    await page.getByRole("button", { name: "Accept proposal" }).click();
    await expect(page.getByText("Revision 3", { exact: true })).toBeVisible();
    await expect(proposalReview.getByText("accepted", { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "History" }).click();
    await expect(page.getByRole("heading", { name: "Revision history" })).toBeVisible();
    await expect(page.getByText("Revision 3", { exact: true }).first()).toBeVisible();
    await capture(page, testInfo, "workspace-wiki-history", 1280, 900, "[data-testid=wiki-page]");
    await page.keyboard.press("Escape");
    await page.getByRole("tab", { name: "Document" }).click();
    const sourceIssueLink = page.getByRole("link", { name: "Open the source issue" });
    await expect(sourceIssueLink).toHaveAttribute(
      "href",
      `/${workspace.slug}/issues/${sourceIssue.identifier}`,
    );
    await sourceIssueLink.click();
    await expect(page).toHaveURL(new RegExp(`/${workspace.slug}/issues/`));
    await page.goBack({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("wiki-page")).toBeVisible({ timeout: 30_000 });
    actions.push("Workspace Wiki operational link resolved inside the workspace");
    await page.emulateMedia({ colorScheme: "dark" });
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("wiki-page").getByRole(
      "heading",
      { name: "Release runbook", exact: true, level: 2 },
    )).toBeVisible({ timeout: 30_000 });
    await capture(page, testInfo, "workspace-wiki-dark", 375, 844, "[data-testid=wiki-page]");
    await page.emulateMedia({ colorScheme: "light" });

    const acceptedPage = await owner.request(`/api/wiki/pages/${sharedPage.id}/`)
      .then(async (response) => {
        expect(response.status).toBe(200);
        return response.json() as Promise<typeof sharedPage>;
      });
    expect(acceptedPage.current_revision_number).toBe(3);
    const sharedRevisions = await owner.listWikiRevisions(sharedPage.id);
    expect(sharedRevisions.map((revision) => revision.revision_number)).toEqual([3, 2, 1]);
    const acceptedWikiRevision = sharedRevisions.find((revision) => revision.revision_number === 3);
    expect(acceptedWikiRevision).toMatchObject({
      id: acceptedPage.current_revision_id,
      source_kind: "agent_proposal",
      content: reviewedContent,
    });
    if (!acceptedWikiRevision) throw new Error("Accepted Workspace Wiki revision r3 is missing");

    const personalPolicy = await owner.request("/api/lm-wiki/source-policy", {
      method: "PUT",
      body: JSON.stringify({
        source_classes: ["issue", "wiki_page"],
        wiki_pages: [{ page_id: personalPage.id, revision_number: 1 }],
        remote_generation_enabled: true,
      }),
    });
    expect(personalPolicy.status).toBe(400);
    const sourcePolicyInput: TestLMWikiSourcePolicyInput = {
      source_classes: ["autopilot_run", "issue", "project", "project_resource", "wiki_page"],
      wiki_pages: [{ page_id: sharedPage.id, revision_number: 3 }],
      remote_generation_enabled: true,
    };
    const sourcePolicy = await owner.updateLMWikiSourcePolicy(sourcePolicyInput);
    expect(sourcePolicy.wiki_pages).toEqual([{ page_id: sharedPage.id, revision_number: 3 }]);
    expect(sourcePolicy.remote_generation_enabled).toBe(true);
    expect(sourcePolicy.policy_version).toBeGreaterThan(0);
    expect(sourcePolicy.policy_digest).toMatch(/^sha256:[0-9a-f]{64}$/);
    expect(sourcePolicy.exclusions).toEqual(expect.arrayContaining([
      expect.objectContaining({ source_class: "personal_wiki", state: "always_excluded" }),
      expect.objectContaining({ source_class: "local_only", state: "always_excluded" }),
    ]));
    const currentPage = await owner.updateWikiPage(sharedPage.id, {
      expected_revision_number: 3,
      content: "# Release runbook\n\nRevision four must not replace the pinned evidence.\n",
    });
    expect(currentPage.current_revision_number).toBe(4);
    actions.push("Workspace Wiki CAS, search, Agent proposal, human edit, and exact source pin verified");

    await page.goto(`/${workspace.slug}/twins`, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "LM Wiki and Twin", exact: true })).toBeVisible({
      timeout: 120_000,
    });
    const sourcePolicyPanel = page.getByTestId("lm-wiki-source-policy");
    await expect(sourcePolicyPanel).toBeVisible({ timeout: 30_000 });
    await expect(sourcePolicyPanel.getByRole("checkbox", { name: "Release runbook" })).toBeChecked();
    await expect(sourcePolicyPanel.getByRole("checkbox", { name: "Private release notes" }))
      .toHaveAttribute("aria-disabled", "true");
    await expect(sourcePolicyPanel.getByText(
      "Personal pages are private and cannot be LM Wiki evidence.",
    )).toBeVisible();
    await expect(sourcePolicyPanel.getByRole("switch", { name: "Allow remote generation" }))
      .toBeChecked();
    const permanentExclusions = sourcePolicyPanel.getByRole("list", {
      name: "Permanent exclusions",
    });
    await expect(permanentExclusions.getByText("Personal Wiki pages")).toBeVisible();
    await expect(permanentExclusions.getByText("Local-only sources")).toBeVisible();
    await sourcePolicyPanel.scrollIntoViewIfNeeded();
    await capture(page, testInfo, "wiki-source-policy", 1280, 900);

    const refreshButton = page.getByRole("button", { name: "Refresh Wiki" });
    await expect(refreshButton).toBeVisible({ timeout: 30_000 });
    await capture(page, testInfo, "initial-wiki", 1280, 900);
    // A daily reconciliation can create a default-off revision before the owner
    // finishes configuring sources. Always refresh explicitly so the candidate
    // under review freezes the just-saved policy, regardless of that race.
    await refreshButton.click();
    actions.push("manager explicitly refreshed Wiki with the saved source policy");
    await expect(page.getByText("Pending review").first()).toBeVisible();

    const generatedWiki = await owner.getLMWiki();
    const generatedRevision = generatedWiki.pending_revision ?? generatedWiki.latest_revision;
    if (!generatedRevision) throw new Error("LM Wiki refresh did not produce a revision");
    const firstWikiRevisionNumber = generatedRevision.revision_number;
    await expect(page.getByRole("heading", {
      name: `Revision r${firstWikiRevisionNumber}`,
    })).toBeVisible();
    actions.push("policy-bound Wiki revision pending");
    const generatedDetail = await owner.getLMWikiRevision(generatedRevision.id);
    expect(generatedDetail.revision.schema_version).toBe(2);
    expect(generatedDetail.revision.content.schema_version).toBe(2);
    expect(generatedDetail.revision.content.egress_policy).toEqual({
      remote_generation_enabled: true,
      policy_version: sourcePolicy.policy_version,
      policy_digest: sourcePolicy.policy_digest,
    });
    expect(generatedDetail.revision).toMatchObject({
      remote_generation_enabled: true,
      source_policy_version: sourcePolicy.policy_version,
      source_policy_digest: sourcePolicy.policy_digest,
    });
    expect(generatedDetail.revision.content.wiki_pages).toEqual([
      expect.objectContaining({
        citation_key: `wiki_page_revision:${acceptedWikiRevision.id}`,
        revision_id: acceptedWikiRevision.id,
        page_id: sharedPage.id,
        revision_number: 3,
        content: reviewedContent,
        content_digest: acceptedWikiRevision.content_digest,
      }),
    ]);
    const canonicalWikiEvidence = generatedDetail.revision.content.wiki_pages[0];
    if (!canonicalWikiEvidence) throw new Error("LM Wiki revision omitted the pinned Wiki evidence");
    if (!canonicalWikiEvidence.created_at) throw new Error("LM Wiki evidence omitted its immutable creation time");
    const canonicalWikiCitationItem = {
      citation_key: canonicalWikiEvidence.citation_key,
      revision_id: canonicalWikiEvidence.revision_id,
      page_id: canonicalWikiEvidence.page_id,
      scope: canonicalWikiEvidence.scope,
      ...(canonicalWikiEvidence.project_id ? { project_id: canonicalWikiEvidence.project_id } : {}),
      revision_number: canonicalWikiEvidence.revision_number,
      path: canonicalWikiEvidence.path,
      title: canonicalWikiEvidence.title,
      content: canonicalWikiEvidence.content,
      content_digest: canonicalWikiEvidence.content_digest,
      created_at: canonicalWikiEvidence.created_at,
    };
    const canonicalWikiDigest = `sha256:${createHash("sha256")
      .update(JSON.stringify(canonicalWikiCitationItem))
      .digest("hex")}`;
    expect(generatedDetail.citations).toContainEqual(expect.objectContaining({
      citation_key: `wiki_page_revision:${acceptedWikiRevision.id}`,
      source_type: "wiki_page_revision",
      source_id: acceptedWikiRevision.id,
      source_digest: canonicalWikiDigest,
    }));
    expect(generatedDetail.revision.content.wiki_pages[0]?.content)
      .not.toContain("Revision four must not replace");
    actions.push("LM Wiki canonical v2 consumed the pinned immutable Wiki r3, not mutable r4");

    const citation = page.getByRole("button", { name: "Show citation" }).first();
    await citation.focus();
    await page.keyboard.press("Enter");
    await expect(page.getByText(/Issue #1: Review the live Wiki source/)).toBeVisible();
    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await expect.poll(async () => (await owner.getLMWiki()).accepted_revision?.id)
      .toBe(generatedRevision.id);
    await page.getByRole("tab", { name: "Twin Builder" }).click();
    await page.getByRole("button", { name: "Build proposal" }).click();
    await expect(page.getByRole("button", { name: "Sign off proposal" })).toBeVisible();
    const initialTwinProposal = (await owner.getTwins()).pending_proposal;
    if (!initialTwinProposal) throw new Error("Explicit Twin build did not create a proposal");
    const replayedInitialProposal = await owner.ensureTwinProposal(generatedRevision.id);
    expect(replayedInitialProposal).toMatchObject({
      created: false,
      proposal: { id: initialTwinProposal.id },
    });
    actions.push("accepted evidence stayed durable before an explicit idempotent Twin build");
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
    await expect(page.getByRole("heading", {
      name: `Revision r${firstWikiRevisionNumber + 1}`,
    })).toBeVisible();

    await owner.createIssue("Make the visible review stale", { status: "in_review" });
    const latestRefresh = await owner.refreshLMWiki();
    expect(latestRefresh.revision.revision_number).toBe(firstWikiRevisionNumber + 2);
    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(page.getByTestId("twin-workspace-content").getByRole("alert"))
      .toHaveText("This review is out of date. Check the latest version and try again.");
    await expect(page.getByRole("heading", {
      name: `Revision r${firstWikiRevisionNumber + 2}`,
    })).toBeVisible();
    actions.push("stale Wiki review surfaced and canonical revision reloaded");

    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await expect.poll(async () => (await owner.getLMWiki()).accepted_revision?.id)
      .toBe(latestRefresh.revision.id);
    await page.getByRole("tab", { name: "Twin Builder" }).click();
    await page.getByRole("button", { name: "Build proposal" }).click();
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
    await expect(page.getByRole("heading", {
      name: `Revision r${firstWikiRevisionNumber + 3}`,
    })).toBeVisible();
    await expect.poll(async () => (await owner.getLMWiki()).pending_revision?.revision_number)
      .toBe(firstWikiRevisionNumber + 3);
    const fourthWikiRevision = (await owner.getLMWiki()).pending_revision;
    if (!fourthWikiRevision) throw new Error("LM Wiki revision r4 is missing");
    await page.getByRole("button", { name: "Accept revision" }).click();
    await page.getByRole("button", { name: "Confirm acceptance" }).click();
    await expect.poll(async () => (await owner.getLMWiki()).accepted_revision?.id)
      .toBe(fourthWikiRevision.id);
    await page.getByRole("tab", { name: "Twin Builder" }).click();
    await page.getByRole("button", { name: "Build proposal" }).click();
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
    await page.context().addCookies([{ name: "multica-locale", value: "en", url: APP_URL }]);

    await member.login(memberEmail, "Twin E2E Member");
    await member.markUserOnboarded();
    await addWorkspaceMember(memberEmail, workspace.id);
    member.setWorkspaceId(workspace.id);
    member.setWorkspaceSlug(workspace.slug);
    const memberToken = member.getToken();
    if (!memberToken) throw new Error("Member login did not return a token");
    memberContext = await browser.newContext({ viewport: { width: 768, height: 1024 } });
    await memberContext.addCookies([{ name: "multica-locale", value: "en", url: APP_URL }]);
    await memberContext.addInitScript((value) => {
      localStorage.setItem("multica_token", value);
      localStorage.setItem("multica:chat:isOpen", "false");
    }, memberToken);
    const memberPage = await memberContext.newPage();
    const memberSignals = collectSignals(memberPage);
    await memberPage.goto(`/${workspace.slug}/twins`, { waitUntil: "domcontentloaded" });
    await expect(memberPage.getByText("Read-only access")).toBeVisible({ timeout: 30_000 });
    await expect(memberPage.getByRole("button", { name: "Refresh Wiki" })).toHaveCount(0);
    await expect(memberPage.getByTestId("lm-wiki-source-policy").getByRole(
      "button",
      { name: "Save source policy" },
    )).toBeDisabled();
    await expect(memberPage.getByTestId("lm-wiki-source-policy").getByRole(
      "switch",
      { name: "Allow remote generation" },
    )).toBeDisabled();
    const deniedPolicy = await member.request("/api/lm-wiki/source-policy", {
      method: "PUT",
      body: JSON.stringify(sourcePolicyInput),
    });
    expect(deniedPolicy.status).toBe(403);
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

    await owner.deleteWikiPage(sharedPage.id);
    const deletedPage = await owner.request(`/api/wiki/pages/${sharedPage.id}/`);
    expect(deletedPage.status).toBe(404);
    const durableRevision = await owner.getStableWikiRevision(acceptedWikiRevision.id);
    expect(durableRevision).toMatchObject({
      id: acceptedWikiRevision.id,
      page_id: sharedPage.id,
      revision_number: 3,
      content: reviewedContent,
      content_digest: acceptedWikiRevision.content_digest,
      source_kind: "agent_proposal",
    });
    await page.goto(`/${workspace.slug}/wiki/revisions/${acceptedWikiRevision.id}`, {
      waitUntil: "domcontentloaded",
      timeout: 120_000,
    });
    await expect(page.getByTestId("wiki-revision-page")).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId("wiki-revision-page").locator("header").getByRole(
      "heading",
      { name: "Release runbook", exact: true },
    )).toBeVisible();
    await expect(page.getByText(
      "Verify rollback ownership, notify support, and record the decision.",
    )).toBeVisible();
    await expect(page.getByRole("link", { name: "Open the source issue" })).toHaveAttribute(
      "href",
      `/${workspace.slug}/issues/${sourceIssue.identifier}`,
    );
    await expect(page.getByText(`wiki_page_revision:${acceptedWikiRevision.id}`)).toBeVisible();
    responsive.push(
      await capture(
        page,
        testInfo,
        "deleted-page-stable-revision",
        1280,
        900,
        "[data-testid=wiki-revision-page]",
      ),
    );
    actions.push("deleted live Wiki page remained reviewable through its immutable revision route");
    const ownerContext = page.context();
    await page.close();

    await owner.cleanup();
    const deletedWorkspaceId = workspace.id;
    await owner.deleteWorkspace(deletedWorkspaceId);
    cleanupCounts = await owner.getWikiTwinArtifactCounts(deletedWorkspaceId);
    expect(cleanupCounts).toEqual(zeroCounts());
    workspace = null;

    personalKnowledgePage = await ownerContext.newPage();
    await authenticate(personalKnowledgePage, ownerToken);
    const personalSignals = collectSignals(personalKnowledgePage);
    await personalKnowledgePage.goto(`/personal-wiki/${personalPage.id}`, {
      waitUntil: "domcontentloaded",
      timeout: 120_000,
    });
    await expect(personalKnowledgePage.getByTestId("personal-wiki-page"))
      .toBeVisible({ timeout: 30_000 });
    await expect(personalKnowledgePage.getByRole(
      "heading",
      { name: "Personal Wiki", exact: true },
    )).toBeVisible();
    await expect(personalKnowledgePage.getByTestId("personal-wiki-page").getByRole(
      "heading",
      { name: "Private release notes", exact: true, level: 2 },
    )).toBeVisible();
    await expect(personalKnowledgePage.getByText(
      "Never leave the personal knowledge boundary.",
    )).toBeVisible();
    responsive.push(
      await capture(
        personalKnowledgePage,
        testInfo,
        "personal-wiki-without-workspace",
        1280,
        900,
        "[data-testid=personal-wiki-page]",
      ),
    );
    await personalKnowledgePage.emulateMedia({ colorScheme: "dark" });
    responsive.push(
      await capture(
        personalKnowledgePage,
        testInfo,
        "personal-wiki-without-workspace-dark",
        375,
        844,
        "[data-testid=personal-wiki-page]",
      ),
    );
    await personalKnowledgePage.emulateMedia({ colorScheme: "light" });
    expect(personalSignals.consoleErrors).toEqual([]);
    expect(personalSignals.pageErrors).toEqual([]);
    expect(personalSignals.requestFailures).toEqual([]);
    await personalKnowledgePage.close();
    personalKnowledgePage = null;
    actions.push("personal Wiki remained readable after the final workspace was deleted");

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
    if (personalKnowledgePage) await personalKnowledgePage.close();
    if (memberContext) await memberContext.close();
    await page.close();
    if (personalPageId) {
      await owner.deletePersonalWikiPage(personalPageId);
      personalPageId = null;
    }
    await owner.cleanup();
    if (workspace) {
      await owner.deleteWorkspace(workspace.id);
      cleanupCounts = await owner.getWikiTwinArtifactCounts(workspace.id);
      expect(cleanupCounts).toEqual(zeroCounts());
    }
    await deleteTestUsers([ownerEmail, memberEmail]);
  }
});
