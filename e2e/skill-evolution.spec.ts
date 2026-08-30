import "./env";

import { expect, test, type Page, type Route } from "@playwright/test";
import { loginAsDefaultWithApi } from "./helpers";

const SKILL_ID = "skill-evolution-e2e";
const PROPOSAL_ID = "proposal-e2e";
const BASE_REVISION_ID = "revision-base-e2e";
const CANDIDATE_REVISION_ID = "revision-candidate-e2e";
const RELEASE_ID = "release-e2e";
const NOW = new Date().toISOString();

const baseProposal = {
  id: PROPOSAL_ID,
  skill_id: SKILL_ID,
  state: "queued",
  base_revision_id: BASE_REVISION_ID,
  base_hash: "sha256:base",
  candidate_revision_id: CANDIDATE_REVISION_ID,
  candidate_hash: "sha256:candidate",
  failure_reason: null,
  stale_reason: null,
  created_at: NOW,
  updated_at: NOW,
};

const configuredLoop = {
  id: "loop-e2e",
  enabled: true,
  mode: "propose",
  cooldown_seconds: 3600,
  minimum_signals: 3,
  max_evidence_refs: 20,
  max_replay_samples: 8,
  max_cost_usd_ticks: 10000,
  policy_version: "v1",
  last_observed_at: null,
  last_proposal_at: NOW,
  next_eligible_at: null,
  updated_at: NOW,
};

const publishedRelease = {
  id: RELEASE_ID,
  skill_id: SKILL_ID,
  proposal_id: PROPOSAL_ID,
  source_release_id: null,
  revision_id: CANDIDATE_REVISION_ID,
  kind: "publish",
  expected_base_hash: "sha256:base",
  pre_hash: "sha256:base",
  post_hash: "sha256:candidate",
  outcome: "succeeded",
  error_code: null,
  created_at: NOW,
  completed_at: NOW,
};

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

function proposalDetail(state: "queued" | "ready" | "published") {
  return {
    proposal: {
      ...baseProposal,
      state,
      updated_at: new Date().toISOString(),
    },
    rationale: {
      observed_pattern: "Reviewers repeatedly request explicit rollback criteria.",
      expected_benefit: "Future runs produce safer rollout plans.",
      regression_risk: "The procedure may become too rigid for low-risk changes.",
    },
    diff: {
      truncated: false,
      omitted_rows: 0,
      metadata: [],
      files: [{
        path: "SKILL.md",
        change: "modified",
        truncated: false,
        omitted_rows: 0,
        rows: [{
          kind: "add",
          old_line: null,
          new_line: 8,
          text: "Record rollback criteria before publishing.",
        }],
      }],
    },
    evidence: [{
      kind: "task_feedback",
      source_id: "task-e2e",
      source_revision_id: null,
      source_state: "accepted",
      digest: "sha256:evidence",
      observed_at: NOW,
    }],
    evaluations: [{
      id: "evaluation-e2e",
      kind: "behavioral_replay",
      result: "passed",
      adapter: "fixture",
      adapter_version: "1",
      policy_version: "v1",
      result_digest: "sha256:evaluation",
      safe_metrics: { sample_count: 4 },
      cost_usd_ticks: 0,
      duration_ms: 12,
      created_at: NOW,
    }],
    reviews: [],
  };
}

async function expectNoHorizontalOverflow(page: Page) {
  const dimensions = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth);
}

test("Web and Desktop shared evolution workflow keeps review actions human-gated", async ({
  page,
}) => {
  const { api, workspaceSlug } = await loginAsDefaultWithApi(page, {
    navigate: false,
  });
  const workspace = (await api.getWorkspaces()).find(
    (candidate) => candidate.slug === workspaceSlug,
  );
  if (!workspace) throw new Error(`Workspace ${workspaceSlug} was not found`);

  let loop: typeof configuredLoop | null = null;
  let proposalState: "absent" | "queued" | "ready" | "published" = "absent";
  let proposalReads = 0;
  let requestStarted = false;
  let overviewReadsAfterRequest = 0;
  let emptyOverviewReadsAfterRequest = 0;
  let releases: Array<Record<string, unknown>> = [];
  const configureBodies: unknown[] = [];
  const requestBodies: unknown[] = [];
  const publishBodies: unknown[] = [];
  const rollbackBodies: unknown[] = [];

  await page.route(`**/api/skills/${SKILL_ID}`, (route) =>
    json(route, {
      id: SKILL_ID,
      workspace_id: workspace.id,
      name: "evidence-review",
      description: "Turn recurring review feedback into guarded procedures.",
      content: "# Evidence review\n",
      config: {},
      created_by: null,
      created_at: NOW,
      updated_at: NOW,
      files: [],
    }),
  );

  await page.route("**/api/skill-evolution/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();

    if (method === "GET" && pathname === `/api/skill-evolution/skills/${SKILL_ID}`) {
      if (requestStarted) {
        overviewReadsAfterRequest += 1;
        if (overviewReadsAfterRequest === 1) {
          emptyOverviewReadsAfterRequest += 1;
        } else if (proposalState === "absent") {
          proposalState = "queued";
        }
      }
      const proposals = proposalState === "absent"
        ? []
        : [{ ...baseProposal, state: proposalState }];
      return json(route, {
        skill: {
          id: SKILL_ID,
          name: "evidence-review",
          bundle_hash: proposalState === "published" ? "sha256:candidate" : "sha256:base",
          ownership: "workspace",
          ownership_reason: "manual_or_unattributed",
          fork_required: false,
        },
        loop,
        revisions: [
          {
            id: BASE_REVISION_ID,
            kind: "base",
            bundle_hash: "sha256:base",
            byte_count: 18,
            support_file_count: 0,
            created_at: NOW,
          },
          ...(proposalState === "published"
            ? [{
                id: CANDIDATE_REVISION_ID,
                kind: "candidate",
                bundle_hash: "sha256:candidate",
                byte_count: 66,
                support_file_count: 0,
                created_at: NOW,
              }]
            : []),
        ],
        proposals,
        releases,
        permissions: {
          can_configure: true,
          can_publish: true,
          can_fork: true,
        },
      });
    }

    if (
      method === "PUT" &&
      pathname === `/api/skill-evolution/skills/${SKILL_ID}/loop`
    ) {
      configureBodies.push(request.postDataJSON());
      loop = configuredLoop;
      return json(route, configuredLoop);
    }

    if (
      method === "POST" &&
      pathname === `/api/skill-evolution/skills/${SKILL_ID}/proposals`
    ) {
      requestBodies.push(request.postDataJSON());
      requestStarted = true;
      return json(
        route,
        {
          state: "improvement_room_queued",
          room_id: "room-e2e",
          proposal: null,
        },
        202,
      );
    }

    if (
      method === "GET" &&
      pathname === `/api/skill-evolution/proposals/${PROPOSAL_ID}`
    ) {
      proposalReads += 1;
      if (proposalState === "queued" && proposalReads > 1) {
        proposalState = "ready";
      }
      return json(route, proposalDetail(proposalState === "absent" ? "queued" : proposalState));
    }

    if (
      method === "POST" &&
      pathname === `/api/skill-evolution/proposals/${PROPOSAL_ID}/publish`
    ) {
      publishBodies.push(request.postDataJSON());
      proposalState = "published";
      releases = [publishedRelease];
      return json(route, {
        proposal: { ...baseProposal, state: "published" },
        release: publishedRelease,
      });
    }

    if (
      method === "POST" &&
      pathname ===
        `/api/skill-evolution/skills/${SKILL_ID}/releases/${RELEASE_ID}/rollback`
    ) {
      rollbackBodies.push(request.postDataJSON());
      const rollbackRelease = {
        ...publishedRelease,
        id: "rollback-e2e",
        proposal_id: null,
        source_release_id: RELEASE_ID,
        kind: "rollback",
        pre_hash: "sha256:candidate",
        post_hash: "sha256:base",
      };
      releases = [rollbackRelease, publishedRelease];
      return json(route, { proposal: null, release: rollbackRelease });
    }

    return json(route, { error: `Unexpected mocked request: ${method} ${pathname}` }, 500);
  });

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/${workspaceSlug}/skills/${SKILL_ID}`, {
    waitUntil: "domcontentloaded",
  });
  await expect(
    page.getByRole("heading", { level: 1, name: "evidence-review", exact: true }),
  ).toBeVisible({ timeout: 30_000 });
  await expectNoHorizontalOverflow(page);

  const evolutionAction = page.getByRole("button", { name: "Evolution" });
  const evolutionPath = `/${workspaceSlug}/skills/${SKILL_ID}/evolution`;
  const evolutionPrefetch = await page.evaluate(async (path) => {
    const response = await fetch(path, {
      headers: {
        "Next-Router-Prefetch": "1",
        RSC: "1",
      },
    });
    return {
      contentType: response.headers.get("content-type"),
      status: response.status,
    };
  }, evolutionPath);
  expect(evolutionPrefetch).toEqual({
    contentType: expect.stringContaining("text/x-component"),
    status: 200,
  });
  await expect(evolutionAction).toHaveAttribute("href", evolutionPath);

  await evolutionAction.hover();
  await evolutionAction.focus();
  await evolutionAction.click();
  await expect(page).toHaveURL(
    new RegExp(`${evolutionPath}$`),
    { timeout: 30_000 },
  );
  await expect(page.getByText("Evolution is off", { exact: true })).toBeVisible();

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.getByRole("switch", { name: "Enabled" }).click();
  await page.getByRole("button", { name: "propose", exact: true }).click();
  await page.getByLabel("Minimum signals").fill("3");
  await page.getByRole("button", { name: "Save configuration" }).click();

  await expect.poll(() => configureBodies.length).toBe(1);
  expect(configureBodies[0]).toEqual({
    enabled: true,
    mode: "propose",
    cooldown_seconds: 3600,
    minimum_signals: 3,
    max_evidence_refs: 20,
    max_replay_samples: 8,
    max_cost_usd_ticks: 10000,
    policy_version: "v1",
  });
  await expect(page.getByText("No proposals", { exact: true })).toBeVisible();

  const requestResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname ===
        `/api/skill-evolution/skills/${SKILL_ID}/proposals`,
  );
  await page.getByRole("button", { name: "Request proposal", exact: true }).click();
  const requestResponse = await requestResponsePromise;
  expect(requestResponse.status()).toBe(202);
  expect(await requestResponse.json()).toEqual({
    state: "improvement_room_queued",
    room_id: "room-e2e",
    proposal: null,
  });
  expect(requestBodies).toHaveLength(1);
  expect(requestBodies[0]).toEqual({
    idempotency_key: expect.stringContaining("skill-evolution-proposal"),
  });
  await expect(page.getByText("Improvement Room queued for this Skill", { exact: true }))
    .toBeVisible();
  await expect(page.getByRole("button", { name: "Open Room", exact: true })).toBeVisible();
  await expect.poll(() => emptyOverviewReadsAfterRequest).toBeGreaterThanOrEqual(1);

  await expect(page.getByText("queued", { exact: true }).first()).toBeVisible({
    timeout: 10_000,
  });
  await expect(page.getByText("Observed pattern", { exact: true })).toBeVisible();
  await expect(page.getByText("ready", { exact: true }).first()).toBeVisible({
    timeout: 10_000,
  });
  await expect(page.getByRole("button", { name: "Publish", exact: true })).toBeVisible({
    timeout: 10_000,
  });

  await page.getByRole("button", { name: "Publish", exact: true }).click();
  const publishDialog = page.getByRole("alertdialog");
  await expect(publishDialog.getByText("Publish this proposal?", { exact: true })).toBeVisible();
  await publishDialog.getByRole("button", { name: "Publish", exact: true }).click();
  await expect.poll(() => publishBodies.length).toBe(1);
  expect(publishBodies[0]).toEqual({
    idempotency_key: expect.stringContaining("skill-evolution-publish"),
  });

  await expect(page.getByRole("button", { name: "Roll back" })).toBeVisible();
  await page.getByRole("button", { name: "Roll back" }).click();
  const rollbackDialog = page.getByRole("alertdialog");
  await expect(rollbackDialog.getByText("Roll back to this release?", { exact: true }))
    .toBeVisible();
  await rollbackDialog.getByRole("button", { name: "Roll back", exact: true }).click();
  await expect.poll(() => rollbackBodies.length).toBe(1);
  expect(rollbackBodies[0]).toEqual({
    idempotency_key: expect.stringContaining("skill-evolution-rollback"),
  });
  await expectNoHorizontalOverflow(page);
});
