import "./env";

import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { expect, test, type Page, type TestInfo } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

const DATABASE_URL =
  process.env.DATABASE_URL ??
  "postgres://multica:multica@localhost:5432/multica?sslmode=disable";
const APP_URL = process.env.PLAYWRIGHT_BASE_URL ?? "http://localhost:3000";

interface BrowserSignals {
  consoleErrors: string[];
  expectedConflicts: string[];
  pageErrors: string[];
  requestFailures: string[];
}

interface RoomSynthesisItem {
  text: string;
  citation_entry_ids: string[];
  confidence: number;
}

interface RoomRecommendation {
  key: string;
  kind: "issue" | "wiki" | "decision";
  title: string;
  body: string;
  rationale: string;
  citation_entry_ids: string[];
  confidence: number;
}

interface RoomSynthesis {
  schema_version: 1;
  summary: string;
  facts: RoomSynthesisItem[];
  decisions: RoomSynthesisItem[];
  open_questions: RoomSynthesisItem[];
  disagreements: RoomSynthesisItem[];
  action_items: RoomSynthesisItem[];
  recommendations: RoomRecommendation[];
  confidence: number;
}

interface RoomDetailResponse {
  room: {
    id: string;
    memory_version: number;
    status: string;
    accepted_memory_revision_id: string | null;
    capability_version: number;
  };
  entries: Array<{
    id: string;
    ordinal: number;
    type: string;
    body: string;
    turn_id: string | null;
  }>;
  cycles: Array<{
    id: string;
    sequence: number;
    status: string;
    phase: string;
    refusal_reason: string | null;
    synthesis_error: { code: string; message: string; retryable: boolean } | null;
    memory_revision_id: string | null;
    expected_max_turns: number;
  }>;
  turns: Array<{
    id: string;
    cycle_id: string;
    status: string;
    turn_kind: string;
    attempt: number;
  }>;
  artifacts: Array<{
    id: string;
    kind: string;
    title: string;
    body: string;
    memory_revision_id: string | null;
    recommendation_key: string | null;
    citation_entry_ids: string[];
  }>;
  memory_revisions: Array<{
    id: string;
    version: number;
    review_status: string;
    corrected_from_revision_id: string | null;
    synthesis: RoomSynthesis;
  }>;
  recommendation_reviews: Array<{
    memory_revision_id: string;
    recommendation_key: string;
    status: string;
    artifact_id: string | null;
  }>;
}

interface RoomMessageResponse {
  cycle: RoomDetailResponse["cycles"][number];
  tasks: string[];
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
    if (
      text.includes("409 (Conflict)") ||
      (text.includes("409 /api/rooms/") && text.includes("/messages"))
    ) {
      signals.expectedConflicts.push(text);
      return;
    }
    signals.consoleErrors.push(text);
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
  await expect(page.locator("[data-room-workspace]")).toBeVisible();
  await page.addStyleTag({ content: "nextjs-portal { display: none !important; }" });
  const dimensions = await page.evaluate(() => ({
    viewport: window.innerWidth,
    documentWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(dimensions.documentWidth).toBeLessThanOrEqual(dimensions.clientWidth);
  const outputDir = process.env.ROOM_EVIDENCE_DIR ?? testInfo.outputDir;
  await mkdir(outputDir, { recursive: true });
  await page.screenshot({
    path: path.join(outputDir, `${label}-${width}x${height}.png`),
    animations: "disabled",
  });
  return dimensions;
}

async function deleteTestUser(db: pg.Client, email: string) {
  await db.query(`DELETE FROM "user" WHERE email = $1`, [email]);
}

async function loadRoom(api: TestApiClient, roomId: string): Promise<RoomDetailResponse> {
  const response = await api.request(`/api/rooms/${roomId}`);
  if (!response.ok) {
    throw new Error(`GET Room failed: ${response.status} ${await response.text()}`);
  }
  return response.json() as Promise<RoomDetailResponse>;
}

// This is the deterministic fake runtime boundary: the real server and
// scheduler consume the same terminal task payload that a daemon would write.
async function completeFakeRuntimeTask(
  db: pg.Client,
  taskId: string,
  output: string,
  runId: string,
) {
  const result = await db.query(
    `UPDATE agent_task_queue
     SET status = 'completed', result = $2::jsonb, session_id = $3,
         work_dir = $4, started_at = now(), completed_at = now()
     WHERE id = $1 AND status IN ('queued', 'dispatched', 'running')`,
    [
      taskId,
      JSON.stringify({ output }),
      `room-e2e-${runId}`,
      `/home/cdd/.cache/rooms-e2e-${runId}`,
    ],
  );
  expect(result.rowCount).toBe(1);
}

async function countRoomTasks(db: pg.Client, roomId: string, turnKind: string) {
  const result = await db.query<{ count: number }>(
    `SELECT count(*)::int AS count
     FROM agent_task_queue task
     JOIN room_turn turn ON turn.id = task.room_turn_id
     WHERE turn.room_id = $1 AND turn.turn_kind = $2`,
    [roomId, turnKind],
  );
  return result.rows[0]?.count ?? 0;
}

async function latestRoomTask(db: pg.Client, roomId: string, turnKind: string) {
  const result = await db.query<{ id: string; attempt: number; status: string }>(
    `SELECT task.id::text, turn.attempt, task.status
     FROM agent_task_queue task
     JOIN room_turn turn ON turn.id = task.room_turn_id
     WHERE turn.room_id = $1 AND turn.turn_kind = $2
     ORDER BY turn.attempt DESC, task.created_at DESC, task.id DESC
     LIMIT 1`,
    [roomId, turnKind],
  );
  return result.rows[0] ?? null;
}

async function openRoomTab(page: Page, name: "Transcript" | "Outcome" | "Activity") {
  const tab = page.getByRole("tab", { name, exact: true });
  await expect(tab).toBeVisible({ timeout: 30_000 });
  await tab.click();
  await expect(tab).toHaveAttribute("aria-selected", "true");
}

function validSynthesis(citations: [string, string], runId: string): RoomSynthesis {
  return {
    schema_version: 1,
    summary: `The council recommends a guarded rollout ${runId}.`,
    facts: [{
      text: "Both participant reports identify the same durable boundary.",
      citation_entry_ids: citations,
      confidence: 0.92,
    }],
    decisions: [{
      text: "Ship the boundary behind an observable staged rollout.",
      citation_entry_ids: citations,
      confidence: 0.88,
    }],
    open_questions: [{
      text: "Which cohort should receive the first rollout?",
      citation_entry_ids: [citations[0]],
      confidence: 0.66,
    }],
    disagreements: [{
      text: "Participants disagree on whether the first cohort should be internal-only.",
      citation_entry_ids: citations,
      confidence: 0.81,
    }],
    action_items: [{
      text: "Define rollback signals before enabling the first cohort.",
      citation_entry_ids: [citations[1]],
      confidence: 0.9,
    }],
    recommendations: [{
      key: "",
      kind: "decision",
      title: `Guarded rollout decision ${runId}`,
      body: "Roll out in stages with explicit rollback signals.",
      rationale: "This preserves both the shared evidence and the recorded dissent.",
      citation_entry_ids: citations,
      confidence: 0.89,
    }],
    confidence: 0.87,
  };
}

test("runs a durable Room outcome loop from paused message through reviewed promotion", async ({ page }, testInfo) => {
  test.setTimeout(600_000);
  page.setDefaultTimeout(30_000);
  const runId = `${Date.now().toString(36)}-${process.pid.toString(36)}-${testInfo.workerIndex}`;
  const email = `room-owner-${runId}@multica.ai`;
  const workspaceSlug = `room-e2e-${runId}`;
  const roomTitle = `Research council ${runId}`;
  const facilitatorName = `Room facilitator ${runId}`;
  const participantName = `Room challenger ${runId}`;
  const unsentDraft = `Unsent Room draft ${runId}`;
  const pausedMessage = `Captured while paused ${runId}`;
  const activeMessage = `Assess the durable Room boundary ${runId}`;
  const participantOutputs = [
    `Evidence A: stage the Room rollout and measure failure signals ${runId}.`,
    `Evidence B: preserve dissent and define rollback criteria ${runId}.`,
  ] as const;
  const malformedOutput = `This is deliberately not structured synthesis JSON ${runId}.`;
  const correctedSummary = `Owner-corrected recommendation for a guarded rollout ${runId}.`;
  const correctedDecision = `Owner confirms a staged rollout with explicit rollback signals ${runId}.`;
  const artifactTitle = `Reviewed Room decision ${runId}`;
  const artifactBody = `Ship to a measured cohort, preserve citations, and roll back on regression ${runId}.`;
  const artifactRationale = `Accepted after human correction ${runId}.`;
  const api = new TestApiClient();
  const db = new pg.Client(DATABASE_URL);
  const signals = collectSignals(page);
  const actions: string[] = [];
  let workspaceId = "";
  let roomId = "";
  const taskIds: string[] = [];
  let testFailed = false;
  let testError: unknown;

  await db.connect();
  try {
    await api.login(email, "Room E2E Owner");
    const workspace = await api.ensureWorkspace(`Room E2E ${runId}`, workspaceSlug);
    workspaceId = workspace.id;
    api.setWorkspaceId(workspace.id);
    api.setWorkspaceSlug(workspace.slug);
    await api.markUserOnboarded();

    const userResult = await db.query<{ id: string }>(
      `SELECT id::text FROM "user" WHERE email = $1`,
      [email],
    );
    const userId = userResult.rows[0]?.id;
    if (!userId) throw new Error("Room E2E owner was not created");

    await db.query(
      `UPDATE workspace
       SET settings = COALESCE(settings, '{}'::jsonb) || '{"room_outcomes_v2":true}'::jsonb
       WHERE id = $1`,
      [workspace.id],
    );
    const runtimeResult = await db.query<{ id: string }>(
      `INSERT INTO agent_runtime (
         workspace_id, owner_id, name, runtime_mode, provider, status,
         visibility, device_info, metadata, last_seen_at
       ) VALUES ($1, $2, $3, 'cloud', 'room_e2e', 'online',
                 'private', 'Room E2E fake runtime',
                 '{"capabilities":["room-tasks-v1","room-outcomes-v2"]}'::jsonb, now())
       RETURNING id::text`,
      [workspace.id, userId, `Room runtime ${runId}`],
    );
    const runtimeId = runtimeResult.rows[0]?.id;
    if (!runtimeId) throw new Error("Room E2E runtime was not created");

    const agentsResult = await db.query<{ id: string; name: string }>(
      `INSERT INTO agent (
         workspace_id, name, description, instructions, runtime_mode,
         runtime_config, runtime_id, visibility, max_concurrent_tasks, owner_id
       ) VALUES
         ($1, $2, 'Room E2E facilitator', '', 'cloud', '{}'::jsonb, $4, 'workspace', 2, $5),
         ($1, $3, 'Room E2E challenger', '', 'cloud', '{}'::jsonb, $4, 'workspace', 2, $5)
       RETURNING id::text, name`,
      [workspace.id, facilitatorName, participantName, runtimeId, userId],
    );
    expect(agentsResult.rows).toHaveLength(2);

    const token = api.getToken();
    if (!token) throw new Error("Room E2E login did not return a token");
    await authenticate(page, token);
    await page.setViewportSize({ width: 1440, height: 960 });
    await page.goto(`${APP_URL}/${workspace.slug}/rooms`, { waitUntil: "domcontentloaded" });
    await expect(page.locator("[data-room-workspace]")).toBeVisible({ timeout: 30_000 });

    await page.getByTestId("room-create-open").click();
    await page.getByLabel("Name").fill(roomTitle);
    await page.getByLabel("Objective").fill("Produce a cited rollout decision with dissent preserved.");
    await page.getByLabel("Success criteria").fill("Every conclusion is cited\nThe owner can accept or correct the result");
    await page.getByLabel("Stop conditions").fill("A reviewed decision is promoted");
    await page.getByLabel("Instructions").fill("Compare evidence and preserve durable decisions.");
    await page.getByText("Select an agent", { exact: true }).click();
    await page.getByRole("option", { name: facilitatorName }).click();
    await page.getByRole("checkbox", { name: participantName }).click();
    const createResponsePromise = page.waitForResponse(
      (response) => response.url().endsWith("/api/rooms") && response.request().method() === "POST",
    );
    await page.getByTestId("room-create-submit").click();
    const createResponse = await createResponsePromise;
    expect(createResponse.status()).toBe(201);
    const created = (await createResponse.json()) as RoomDetailResponse;
    roomId = created.room.id;
    expect(created.room.capability_version).toBe(2);
    await expect(page.getByTestId(`room-list-item-${roomId}`)).toContainText(roomTitle);
    actions.push("created a v2 Room with a capable facilitator and challenger");

    await expect(page.locator('[data-sonner-toast]')).toBeHidden({ timeout: 10_000 });
    const starter = page.getByTestId("room-starter-unblock");
    await expect(page.locator('[data-testid^="room-starter-"]')).toHaveCount(3);
    await starter.click();
    await expect(page.getByTestId("room-message-input")).not.toHaveValue("");
    await expect(page.getByTestId("room-message-input")).toBeFocused();
    await page.getByTestId("room-message-input").fill("");

    await test.step("restore an unsent Room draft after reload", async () => {
      const messageInput = page.getByTestId("room-message-input");
      await messageInput.fill(unsentDraft);
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(page.locator("[data-room-workspace]")).toBeVisible({ timeout: 30_000 });
      await expect(page.getByTestId("room-message-input")).toHaveValue(unsentDraft);
    });
    actions.push("restored an unsent Room draft after reload");

    await test.step("pause the room and persist a refused message", async () => {
      const statusToggle = page.getByTestId("room-status-toggle");
      const statusResponsePromise = page.waitForResponse(
        (response) =>
          response.url().endsWith(`/api/rooms/${roomId}/status`) &&
          response.request().method() === "PUT",
      );
      await statusToggle.click();
      expect((await statusResponsePromise).status()).toBe(200);
      await expect(page.getByText("Paused", { exact: true }).first()).toBeVisible();
    });
    const pausedResponsePromise = page.waitForResponse(
      (response) =>
        response.url().endsWith(`/api/rooms/${roomId}/messages`) &&
        response.request().method() === "POST",
    );
    await page.getByTestId("room-message-input").fill(pausedMessage);
    await page.getByTestId("room-message-send").click();
    const pausedResponse = await pausedResponsePromise;
    expect(pausedResponse.status()).toBe(409);
    const refused = (await pausedResponse.json()) as RoomMessageResponse;
    expect(refused.cycle.refusal_reason).toBe("room_paused");
    await expect(page.getByText(pausedMessage, { exact: true })).toBeVisible({ timeout: 15_000 });
    await expect(
      page.locator('[data-sonner-toast][data-type="warning"]').filter({
        hasText: "Message was saved, but execution did not start",
      }),
    ).toBeVisible();
    await expect(page.getByTestId("room-wake")).toBeDisabled();
    const pausedCounts = await db.query<{ entries: number; tasks: number }>(
      `SELECT
         (SELECT count(*)::int FROM room_entry WHERE room_id = $1 AND body = $2) AS entries,
         (SELECT count(*)::int FROM agent_task_queue q
          JOIN room_turn t ON t.id = q.room_turn_id WHERE t.room_id = $1) AS tasks`,
      [roomId, pausedMessage],
    );
    expect(pausedCounts.rows[0]).toEqual({ entries: 1, tasks: 0 });
    actions.push("saved a paused message with a refusal and no task");

    const resumeResponsePromise = page.waitForResponse(
      (response) =>
        response.url().endsWith(`/api/rooms/${roomId}/status`) &&
        response.request().method() === "PUT",
    );
    await page.getByTestId("room-status-toggle").click();
    expect((await resumeResponsePromise).status()).toBe(200);
    await expect(page.getByText("Active", { exact: true }).first()).toBeVisible();

    await page.getByRole("button", { name: "Notify agents" }).click();
    await page.getByRole("menuitemcheckbox", { name: facilitatorName }).click();
    await page.getByRole("menuitemcheckbox", { name: participantName }).click();
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("room-mention-count")).toHaveText("2");

    const activeResponsePromise = page.waitForResponse(
      (response) =>
        response.url().endsWith(`/api/rooms/${roomId}/messages`) &&
        response.request().method() === "POST",
    );
    await page.getByTestId("room-message-input").fill(activeMessage);
    await page.getByTestId("room-message-send").click();
    const activeResponse = await activeResponsePromise;
    expect(activeResponse.status()).toBe(201);
    const activePayload = (await activeResponse.json()) as RoomMessageResponse;
    expect(activePayload.cycle.phase).toBe("gathering");
    expect(activePayload.cycle.expected_max_turns).toBe(3);
    expect(activePayload.tasks).toHaveLength(2);
    taskIds.push(...activePayload.tasks);
    actions.push("queued two participant turns behind the v2 preflight barrier");

    await completeFakeRuntimeTask(db, activePayload.tasks[0]!, participantOutputs[0], runId);
    await completeFakeRuntimeTask(db, activePayload.tasks[1]!, participantOutputs[1], runId);

    let participantEntryIds: [string, string] = ["", ""];
    await expect
      .poll(
        async () => {
          const detail = await loadRoom(api, roomId);
          const participantEntries = detail.entries
            .filter((entry) => participantOutputs.includes(entry.body as (typeof participantOutputs)[number]))
            .sort((left, right) => left.ordinal - right.ordinal);
          if (participantEntries.length === 2) {
            participantEntryIds = [participantEntries[0]!.id, participantEntries[1]!.id];
          }
          return {
            entries: participantEntries.length,
            phase: detail.cycles.find((cycle) => cycle.id === activePayload.cycle.id)?.phase,
            synthesisTurns: detail.turns.filter(
              (turn) => turn.cycle_id === activePayload.cycle.id && turn.turn_kind === "synthesis",
            ).length,
          };
        },
        { timeout: 90_000, intervals: [1_000, 2_000, 5_000] },
      )
      .toEqual({ entries: 2, phase: "synthesizing", synthesisTurns: 1 });
    expect(participantEntryIds.every(Boolean)).toBe(true);

    const initialSynthesisTask = await latestRoomTask(db, roomId, "synthesis");
    if (!initialSynthesisTask) throw new Error("Initial synthesis task was not created");
    taskIds.push(initialSynthesisTask.id);
    await completeFakeRuntimeTask(db, initialSynthesisTask.id, malformedOutput, runId);
    await expect
      .poll(
        async () => {
          const detail = await loadRoom(api, roomId);
          const cycle = detail.cycles.find((candidate) => candidate.id === activePayload.cycle.id);
          return {
            phase: cycle?.phase,
            code: cycle?.synthesis_error?.code,
            revisions: detail.memory_revisions.length,
          };
        },
        { timeout: 90_000, intervals: [1_000, 2_000, 5_000] },
      )
      .toEqual({ phase: "awaiting_review", code: "malformed_synthesis", revisions: 0 });

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.locator("[data-room-workspace]")).toBeVisible({ timeout: 30_000 });
    await openRoomTab(page, "Outcome");
    await expect(page.getByText("Synthesis needs attention", { exact: true })).toBeVisible();
    await expect(page.getByText("Synthesis attempt 1", { exact: true })).toBeVisible();

    const participantTasksBeforeRetry = await countRoomTasks(db, roomId, "participant");
    const synthesisTasksBeforeRetry = await countRoomTasks(db, roomId, "synthesis");
    await db.query(
      `UPDATE agent_runtime SET status = 'online', last_seen_at = now() WHERE id = $1`,
      [runtimeId],
    );
    const retryResponsePromise = page.waitForResponse(
      (response) =>
        response.url().endsWith(`/api/rooms/${roomId}/cycles/${activePayload.cycle.id}/synthesis/retry`) &&
        response.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Retry synthesis", exact: true }).click();
    const retryResponse = await retryResponsePromise;
    const retryPayload = (await retryResponse.json()) as { task_id?: string; error?: string; code?: string };
    expect([200, 202], JSON.stringify(retryPayload)).toContain(retryResponse.status());
    expect(retryPayload.task_id).toBeTruthy();
    if (!retryPayload.task_id) throw new Error("Synthesis retry did not return a task id");
    taskIds.push(retryPayload.task_id);
    expect(await countRoomTasks(db, roomId, "participant")).toBe(participantTasksBeforeRetry);
    expect(await countRoomTasks(db, roomId, "synthesis")).toBe(synthesisTasksBeforeRetry + 1);
    actions.push("retried only synthesis while preserving both participant tasks");

    const synthesis = validSynthesis(participantEntryIds, runId);
    await completeFakeRuntimeTask(db, retryPayload.task_id, JSON.stringify(synthesis), runId);
    let pendingRevisionId = "";
    await expect
      .poll(
        async () => {
          const detail = await loadRoom(api, roomId);
          const revision = detail.memory_revisions.find((candidate) => candidate.version === 1);
          pendingRevisionId = revision?.id ?? "";
          return {
            phase: detail.cycles.find((cycle) => cycle.id === activePayload.cycle.id)?.phase,
            status: revision?.review_status,
            disagreements: revision?.synthesis.disagreements.length,
          };
        },
        { timeout: 90_000, intervals: [1_000, 2_000, 5_000] },
      )
      .toEqual({ phase: "awaiting_review", status: "pending", disagreements: 1 });
    expect(pendingRevisionId).not.toBe("");

    await page.reload({ waitUntil: "domcontentloaded" });
    await openRoomTab(page, "Outcome");
    await expect(page.getByText(synthesis.summary, { exact: true })).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText(synthesis.disagreements[0]!.text, { exact: true })).toBeVisible();
    const citation = page.getByRole("button", { name: /Open citation #/ }).first();
    await citation.click();
    await expect(page.locator('[data-testid^="room-entry-"]:focus')).toBeVisible();
    await openRoomTab(page, "Outcome");

    await page.getByTestId("room-correct-outcome").click();
    const correctionDialog = page.getByRole("dialog", { name: "Correct outcome" });
    await expect(correctionDialog).toBeVisible();
    const summaryInput = correctionDialog.getByRole("textbox", { name: "Summary", exact: true });
    const decisionInput = correctionDialog.getByRole("textbox", { name: "Decisions 1", exact: true });
    await summaryInput.fill(correctedSummary);
    await decisionInput.fill(correctedDecision);
    await expect(summaryInput).toHaveValue(correctedSummary);
    await expect(decisionInput).toHaveValue(correctedDecision);
    const correctResponsePromise = page.waitForResponse(
      (response) =>
        response.url().endsWith(`/api/rooms/${roomId}/cycles/${activePayload.cycle.id}/review`) &&
        response.request().method() === "POST",
    );
    await correctionDialog.getByTestId("room-submit-correction").click();
    expect((await correctResponsePromise).status()).toBe(200);

    let correctedRevisionId = "";
    let recommendationKey = "";
    await expect
      .poll(
        async () => {
          const detail = await loadRoom(api, roomId);
          const corrected = detail.memory_revisions.find((revision) => revision.version === 2);
          correctedRevisionId = corrected?.id ?? "";
          recommendationKey = corrected?.synthesis.recommendations[0]?.key ?? "";
          const original = detail.memory_revisions.find((revision) => revision.id === pendingRevisionId);
          return {
            originalStatus: original?.review_status,
            correctedStatus: corrected?.review_status,
            correctedFrom: corrected?.corrected_from_revision_id,
            summary: corrected?.synthesis.summary,
          };
        },
        { timeout: 30_000, intervals: [500, 1_000, 2_000] },
      )
      .toEqual({
        originalStatus: "corrected",
        correctedStatus: "pending",
        correctedFrom: pendingRevisionId,
        summary: correctedSummary,
      });
    expect(correctedRevisionId).not.toBe("");
    expect(recommendationKey).toMatch(/^sha256:[0-9a-f]{64}$/);
    await expect(page.getByText(correctedSummary, { exact: true })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByTestId("room-accept-outcome")).toBeVisible();

    const acceptResponsePromise = page.waitForResponse(
      (response) =>
        response.url().endsWith(`/api/rooms/${roomId}/cycles/${activePayload.cycle.id}/review`) &&
        response.request().method() === "POST",
    );
    await page.getByTestId("room-accept-outcome").click();
    expect((await acceptResponsePromise).status()).toBe(200);
    await expect(
      page.getByTestId("room-outcome").getByText("Accepted", { exact: true }),
    ).toBeVisible({ timeout: 15_000 });

    const accepted = await loadRoom(api, roomId);
    expect(accepted.room.accepted_memory_revision_id).toBe(correctedRevisionId);
    expect(accepted.room.memory_version).toBe(1);
    expect(accepted.cycles.find((cycle) => cycle.id === activePayload.cycle.id)).toMatchObject({
      phase: "completed",
      status: "completed",
    });
    actions.push("corrected a cited outcome and explicitly accepted the new pending revision");

    await page.getByTestId(`room-approve-recommendation-${recommendationKey}`).click();
    const promotionDialog = page.getByRole("dialog", { name: "Promote result" });
    await expect(promotionDialog).toBeVisible();
    await expect(promotionDialog.getByTestId("room-promotion-kind")).toBeDisabled();
    const titleInput = promotionDialog.getByRole("textbox", { name: "Title", exact: true });
    const bodyInput = promotionDialog.getByRole("textbox", { name: "Body", exact: true });
    const rationaleInput = promotionDialog.getByRole("textbox", { name: "Rationale", exact: true });
    await titleInput.fill(artifactTitle);
    await bodyInput.fill(artifactBody);
    await rationaleInput.fill(artifactRationale);
    await expect(titleInput).toHaveValue(artifactTitle);
    await expect(bodyInput).toHaveValue(artifactBody);
    await expect(rationaleInput).toHaveValue(artifactRationale);
    const promoteResponsePromise = page.waitForResponse(
      (response) =>
        response.url().endsWith(`/api/rooms/${roomId}/promotions`) &&
        response.request().method() === "POST",
    );
    await promotionDialog.getByTestId("room-promote-submit").click();
    const promoteResponse = await promoteResponsePromise;
    expect(promoteResponse.status()).toBe(201);
    const promotionRequest = promoteResponse.request().postDataJSON() as Record<string, unknown>;
    const artifact = (await promoteResponse.json()) as RoomDetailResponse["artifacts"][number];
    await expect(page.getByTestId(`room-artifact-${artifact.id}`)).toContainText(artifactTitle, {
      timeout: 15_000,
    });

    const replayResponse = await api.request(`/api/rooms/${roomId}/promotions`, {
      method: "POST",
      body: JSON.stringify(promotionRequest),
    });
    expect(replayResponse.status).toBe(200);
    const replayedArtifact = (await replayResponse.json()) as RoomDetailResponse["artifacts"][number];
    expect(replayedArtifact.id).toBe(artifact.id);
    const provenance = await db.query<{
      count: number;
      memory_revision_id: string;
      recommendation_key: string;
      citation_entry_ids: string[];
      title: string;
      body: string;
    }>(
      `SELECT count(*) OVER ()::int AS count,
              memory_revision_id::text, recommendation_key, citation_entry_ids,
              title, body
       FROM room_artifact
       WHERE room_id = $1 AND idempotency_key = $2`,
      [roomId, promotionRequest.idempotency_key],
    );
    expect(provenance.rows).toHaveLength(1);
    expect(provenance.rows[0]).toEqual({
      count: 1,
      memory_revision_id: correctedRevisionId,
      recommendation_key: recommendationKey,
      citation_entry_ids: participantEntryIds,
      title: artifactTitle,
      body: artifactBody,
    });
    actions.push("promoted an edited recommendation once with accepted-revision provenance");

    const responsive = [await capture(page, testInfo, "room-outcome-desktop-light", 1440, 960)];

    await test.step("surface realtime activity while reading history", async () => {
      await openRoomTab(page, "Transcript");
      const pauseResponsePromise = page.waitForResponse(
        (response) =>
          response.url().endsWith(`/api/rooms/${roomId}/status`) &&
          response.request().method() === "PUT",
      );
      await page.getByTestId("room-status-toggle").click();
      expect((await pauseResponsePromise).status()).toBe(200);

      const messageInput = page.getByTestId("room-message-input");
      const entriesBefore = await page.locator('[data-testid^="room-entry-"]').count();
      for (let index = 0; index < 8; index += 1) {
        const responsePromise = page.waitForResponse(
          (response) =>
            response.url().endsWith(`/api/rooms/${roomId}/messages`) &&
            response.request().method() === "POST",
        );
        await messageInput.fill(`Background note ${index + 1} ${runId}`);
        await messageInput.press("Enter");
        expect((await responsePromise).status()).toBe(409);
        await expect(messageInput).toHaveValue("");
      }
      await expect(page.locator('[data-testid^="room-entry-"]')).toHaveCount(entriesBefore + 8, {
        timeout: 15_000,
      });
      const transcript = page.getByTestId("room-transcript");
      await transcript.evaluate((element) => {
        element.scrollTop = 0;
        element.dispatchEvent(new Event("scroll"));
      });

      const liveResponsePromise = page.waitForResponse(
        (response) =>
          response.url().endsWith(`/api/rooms/${roomId}/messages`) &&
          response.request().method() === "POST",
      );
      await messageInput.fill(`New while reading history ${runId}`);
      await messageInput.press("Enter");
      expect((await liveResponsePromise).status()).toBe(409);
      const newEntries = page.getByTestId("room-transcript-new-entries");
      await expect(newEntries).toContainText("1 new");
      await expect(newEntries).toHaveAccessibleName("Show 1 new updates");
      await newEntries.click();
      await expect(newEntries).toBeHidden();
      await expect(transcript).toBeFocused();
      const distanceFromLatest = await transcript.evaluate(
        (element) => element.scrollHeight - element.clientHeight - element.scrollTop,
      );
      expect(distanceFromLatest).toBeLessThanOrEqual(1);
    });
    actions.push("surfaced unseen realtime activity and returned to the live edge");

    await test.step("keep a longer Room list within the mobile viewport budget", async () => {
      await page.setViewportSize({ width: 1440, height: 960 });
      for (let index = 1; index <= 5; index += 1) {
        await page.getByTestId("room-create-open").click();
        await page.getByLabel("Name").fill(`${roomTitle} ${index + 1}`);
        await page.getByText("Select an agent", { exact: true }).click();
        await page.getByRole("option", { name: facilitatorName }).click();
        const responsePromise = page.waitForResponse(
          (response) =>
            response.url().endsWith("/api/rooms") && response.request().method() === "POST",
        );
        await page.getByTestId("room-create-submit").click();
        expect((await responsePromise).status()).toBe(201);
      }
      await page.setViewportSize({ width: 390, height: 844 });
      await expect(page.locator('[data-testid^="room-list-item-"]')).toHaveCount(6);
      const listMetrics = await page.getByTestId("room-list-scroll").evaluate((element) => ({
        clientHeight: element.clientHeight,
        scrollHeight: element.scrollHeight,
      }));
      const cappedList = await page.getByTestId("room-list").boundingBox();
      expect(cappedList?.height).toBeLessThanOrEqual(Math.ceil(844 * 0.3) + 1);
      expect(listMetrics.scrollHeight).toBeGreaterThan(listMetrics.clientHeight);
    });
    actions.push("kept the long mobile Room list capped and scrollable");

    await page.emulateMedia({ colorScheme: "dark", reducedMotion: "reduce" });
    await page.context().addCookies([
      { name: "multica-locale", value: "zh-Hans", url: APP_URL },
    ]);
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.locator("[data-room-workspace]")).toBeVisible({ timeout: 30_000 });
    await page.getByTestId(`room-list-item-${roomId}`).click();
    const zhOutcomeTab = page.getByRole("tab", { name: "结果", exact: true });
    if (await zhOutcomeTab.isVisible()) await zhOutcomeTab.click();
    await expect(page.getByText(correctedSummary, { exact: true })).toBeVisible();
    responsive.push(
      await capture(page, testInfo, "room-outcome-tablet-dark-zh", 768, 900),
      await capture(page, testInfo, "room-outcome-mobile-dark-zh", 390, 844),
    );
    actions.push("verified the reviewed outcome in dark CJK tablet and mobile layouts");

    expect(signals.consoleErrors).toEqual([]);
    expect(signals.pageErrors).toEqual([]);
    expect(signals.requestFailures).toEqual([]);
    expect(signals.expectedConflicts.length).toBeGreaterThanOrEqual(1);
    const outputDir = process.env.ROOM_EVIDENCE_DIR ?? testInfo.outputDir;
    await writeFile(
      path.join(outputDir, "browser-actions.json"),
      JSON.stringify({ actions, responsive, signals }, null, 2),
    );
  } catch (error) {
    testFailed = true;
    testError = error;
  } finally {
    const cleanupErrors: unknown[] = [];
    const attempt = async (operation: () => Promise<void>) => {
      try {
        await operation();
      } catch (error) {
        cleanupErrors.push(error);
      }
    };

    await attempt(() => page.close());
    await attempt(() => api.cleanup());
    if (workspaceId) {
      await attempt(() => api.deleteWorkspace(workspaceId));
      await attempt(async () => {
        const cleanup = await db.query<{
          rooms: number;
          entries: number;
          cycles: number;
          turns: number;
          revisions: number;
          artifacts: number;
          tasks: number;
        }>(
          `SELECT
             (SELECT count(*)::int FROM room WHERE workspace_id = $1) AS rooms,
             (SELECT count(*)::int FROM room_entry WHERE workspace_id = $1) AS entries,
             (SELECT count(*)::int FROM room_cycle WHERE workspace_id = $1) AS cycles,
             (SELECT count(*)::int FROM room_turn WHERE workspace_id = $1) AS turns,
             (SELECT count(*)::int FROM room_memory_revision WHERE workspace_id = $1) AS revisions,
             (SELECT count(*)::int FROM room_artifact WHERE workspace_id = $1) AS artifacts,
             (SELECT count(*)::int FROM agent_task_queue WHERE id = ANY($2::uuid[])) AS tasks`,
          [workspaceId, taskIds],
        );
        expect(cleanup.rows[0]).toEqual({
          rooms: 0,
          entries: 0,
          cycles: 0,
          turns: 0,
          revisions: 0,
          artifacts: 0,
          tasks: 0,
        });
      });
    }
    await attempt(() => deleteTestUser(db, email));
    await attempt(() => db.end());

    if (testFailed) throw testError;
    if (cleanupErrors.length > 0) throw cleanupErrors[0];
  }
});
