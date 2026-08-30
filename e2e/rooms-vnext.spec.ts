import "./env";

import { mkdir } from "node:fs/promises";
import path from "node:path";
import { expect, test, type Page, type TestInfo } from "@playwright/test";
import pg from "pg";
import { TestApiClient } from "./fixtures";

const DATABASE_URL =
  process.env.DATABASE_URL ??
  "postgres://multica:multica@localhost:5432/multica?sslmode=disable";
const APP_URL =
  process.env.PLAYWRIGHT_BASE_URL ??
  process.env.FRONTEND_ORIGIN ??
  "http://localhost:3000";

interface RoomDetailResponse {
  room: {
    id: string;
    status: string;
    title: string;
    objective: string;
    schedule_interval_minutes: number | null;
  };
  entries: unknown[];
  cycles: unknown[];
  turns: unknown[];
  memory_revisions: unknown[];
  recommendation_reviews: unknown[];
  artifacts: unknown[];
}

interface SeededOutcome {
  readonly cycleId: string;
  readonly revisionId: string;
}

function watchBrowserFailures(page: Page) {
  const pageErrors: string[] = [];
  const requestFailures: string[] = [];
  const consoleErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("requestfailed", (request) => {
    if (request.url().includes("/api/client-usage")) return;
    const failure = request.failure()?.errorText ?? "unknown";
    if (request.method() === "GET" && failure.includes("ERR_ABORTED")) {
      const target = new URL(request.url());
      if (target.origin === new URL(APP_URL).origin && !target.pathname.startsWith("/api/")) {
        return;
      }
    }
    requestFailures.push(
      `${request.method()} ${request.url()} ${failure}`,
    );
  });
  page.on("console", (message) => {
    if (message.type() !== "error") return;
    const value = message.text();
    if (
      value.includes("409 (Conflict)") ||
      (value.includes("409") && value.includes("/api/rooms/"))
    ) {
      return;
    }
    consoleErrors.push(value);
  });
  return { pageErrors, requestFailures, consoleErrors };
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
  const dimensions = await page.evaluate(() => ({
    documentWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(dimensions.documentWidth).toBeLessThanOrEqual(dimensions.clientWidth);
  const outputDir = process.env.ROOM_EVIDENCE_DIR ?? testInfo.outputDir;
  await mkdir(outputDir, { recursive: true });
  await page.screenshot({
    path: path.join(outputDir, `${label}-${width}x${height}.png`),
    animations: "disabled",
    timeout: 90_000,
  });
}

async function seedAcceptedOutcome(
  db: pg.Client,
  input: {
    readonly workspaceId: string;
    readonly roomId: string;
    readonly agentId: string;
    readonly runtimeId: string;
    readonly ownerId: string;
  },
): Promise<SeededOutcome> {
  await db.query("BEGIN");
  try {
    const cycle = await db.query<{ id: string }>(
      `INSERT INTO room_cycle (
         workspace_id, room_id, sequence, source, wake_key, status, phase,
         expected_max_turns, started_at, completed_at, created_at
       ) VALUES (
         $1, $2, 1, 'manual', 'manual:vnext-seeded', 'completed', 'completed',
         2, now() - interval '4 minutes', now() - interval '2 minutes',
         now() - interval '5 minutes'
       ) RETURNING id::text`,
      [input.workspaceId, input.roomId],
    );
    const cycleId = cycle.rows[0]?.id;
    if (!cycleId) throw new Error("Room vNext cycle seed failed");

    const turn = await db.query<{ id: string }>(
      `INSERT INTO room_turn (
         workspace_id, room_id, cycle_id, agent_id, turn_kind, attempt,
         status, started_at, completed_at
       ) VALUES (
         $1, $2, $3, $4, 'synthesis', 1, 'completed',
         now() - interval '4 minutes', now() - interval '2 minutes'
       ) RETURNING id::text`,
      [input.workspaceId, input.roomId, cycleId, input.agentId],
    );
    const turnId = turn.rows[0]?.id;
    if (!turnId) throw new Error("Room vNext turn seed failed");

    const task = await db.query<{ id: string }>(
      `INSERT INTO agent_task_queue (
         agent_id, runtime_id, status, context, room_turn_id, attempt,
         started_at, completed_at
       ) VALUES (
         $1, $2, 'completed', '{}'::jsonb, $3, 1,
         now() - interval '4 minutes', now() - interval '2 minutes'
       ) RETURNING id::text`,
      [input.agentId, input.runtimeId, turnId],
    );
    const taskId = task.rows[0]?.id;
    if (!taskId) throw new Error("Room vNext task seed failed");
    await db.query(
      `INSERT INTO task_usage (task_id, provider, model, cost_usd_ticks)
       VALUES ($1, 'room-vnext-e2e', 'deterministic', 18)`,
      [taskId],
    );

    const synthesis = {
      schema_version: 1,
      summary: "The recurring planning review produced an accepted owner-ready plan.",
      facts: [],
      decisions: [],
      open_questions: [],
      disagreements: [],
      action_items: [],
      recommendations: [],
      confidence: 0.9,
    };
    const revision = await db.query<{ id: string }>(
      `INSERT INTO room_memory_revision (
         workspace_id, room_id, cycle_id, synthesis_turn_id, version,
         schema_version, synthesis, digest, review_status,
         reviewed_by_user_id, reviewed_at, creator_type, creator_id, created_at
       ) VALUES (
         $1, $2, $3, $4, 1, 1, $5::jsonb,
         'sha256:' || repeat('0', 64), 'accepted', $6,
         now() - interval '1 minute', 'agent', $7,
         now() - interval '3 minutes'
       ) RETURNING id::text`,
      [
        input.workspaceId,
        input.roomId,
        cycleId,
        turnId,
        JSON.stringify(synthesis),
        input.ownerId,
        input.agentId,
      ],
    );
    const revisionId = revision.rows[0]?.id;
    if (!revisionId) throw new Error("Room vNext revision seed failed");

    await db.query(
      `UPDATE room_cycle
       SET synthesis_turn_id = $1, memory_revision_id = $2
       WHERE id = $3`,
      [turnId, revisionId, cycleId],
    );
    await db.query(
      `UPDATE room
       SET accepted_memory_revision_id = $1,
           memory = $2::jsonb,
           memory_version = 1,
           last_cycle_sequence = 1,
           last_memory_revision_version = 1,
           active_cycle_id = NULL,
           updated_at = now()
       WHERE id = $3`,
      [
        revisionId,
        JSON.stringify({
          summary: synthesis.summary,
          facts: [],
          decisions: [],
          open_questions: [],
          recent_contributions: [],
        }),
        input.roomId,
      ],
    );
    await db.query("COMMIT");
    return { cycleId, revisionId };
  } catch (error) {
    await db.query("ROLLBACK");
    throw error;
  }
}

test("template creation, attention route, safe reuse, and value review", async ({
  page,
}, testInfo) => {
  test.setTimeout(300_000);
  page.setDefaultTimeout(30_000);
  page.setDefaultNavigationTimeout(120_000);
  const runId = `${Date.now().toString(36)}-${process.pid.toString(36)}-${testInfo.workerIndex}`;
  const email = `room-vnext-${runId}@multica.ai`;
  const workspaceSlug = `room-vnext-${runId}`;
  const facilitatorName = `Outcome facilitator ${runId}`;
  const participantName = `Evidence participant ${runId}`;
  const roomTitle = `Weekly outcome review ${runId}`;
  const customObjective = `Turn week ${runId} into an owner-ready plan.`;
  const api = new TestApiClient();
  const db = new pg.Client(DATABASE_URL);
  const browserFailures = watchBrowserFailures(page);
  let workspaceId = "";

  await db.connect();
  try {
    await api.login(email, "Room vNext Owner");
    const workspace = await api.ensureWorkspace(`Room vNext ${runId}`, workspaceSlug);
    workspaceId = workspace.id;
    api.setWorkspaceId(workspace.id);
    api.setWorkspaceSlug(workspace.slug);
    await api.markUserOnboarded();

    const owner = await db.query<{ id: string }>(
      `SELECT id::text FROM "user" WHERE email = $1`,
      [email],
    );
    const ownerId = owner.rows[0]?.id;
    if (!ownerId) throw new Error("Room vNext owner was not created");

    await db.query(
      `UPDATE workspace
       SET settings = COALESCE(settings, '{}'::jsonb) || '{"room_outcomes_v2":true}'::jsonb
       WHERE id = $1`,
      [workspace.id],
    );
    const runtime = await db.query<{ id: string }>(
      `INSERT INTO agent_runtime (
         workspace_id, owner_id, name, runtime_mode, provider, status,
         visibility, device_info, metadata, last_seen_at
       ) VALUES ($1, $2, $3, 'cloud', 'room_vnext_e2e', 'online',
                 'private', 'Room vNext fake runtime',
                 '{"capabilities":["room-tasks-v1","room-outcomes-v2"]}'::jsonb, now())
       RETURNING id::text`,
      [workspace.id, ownerId, `Room vNext runtime ${runId}`],
    );
    const runtimeId = runtime.rows[0]?.id;
    if (!runtimeId) throw new Error("Room vNext runtime was not created");

    const agents = await db.query<{ id: string; name: string }>(
      `INSERT INTO agent (
         workspace_id, name, description, instructions, runtime_mode,
         runtime_config, runtime_id, visibility, max_concurrent_tasks, owner_id
       ) VALUES
         ($1, $2, 'Room facilitator', '', 'cloud', '{}'::jsonb, $4, 'workspace', 2, $5),
         ($1, $3, 'Room participant', '', 'cloud', '{}'::jsonb, $4, 'workspace', 2, $5)
       RETURNING id::text, name`,
      [workspace.id, facilitatorName, participantName, runtimeId, ownerId],
    );
    const facilitatorId = agents.rows.find((agent) => agent.name === facilitatorName)?.id;
    if (!facilitatorId) throw new Error("Room vNext facilitator was not created");

    const token = api.getToken();
    if (!token) throw new Error("Room vNext login did not return a token");
    await authenticate(page, token);
    await page.setViewportSize({ width: 1440, height: 960 });
    await page.goto(`${APP_URL}/${workspace.slug}/rooms`, {
      waitUntil: "domcontentloaded",
    });
    await expect(page.locator("[data-room-workspace]")).toBeVisible({
      timeout: 30_000,
    });

    await test.step("create from an outcome template without losing edits", async () => {
      await page.getByTestId("room-create-open").click();
      await expect(page.getByRole("dialog")).toBeVisible();
      const templateCards = page.locator('[data-testid^="room-template-"]');
      await expect(templateCards).toHaveCount(6);
      expect(
        await templateCards.evaluateAll((cards) =>
          cards.map((card) => card.getAttribute("data-testid")),
        ),
      ).toEqual([
        "room-template-research",
        "room-template-planning",
        "room-template-risk",
        "room-template-incident",
        "room-template-decision",
        "room-template-improvement",
      ]);
      await expect(page.getByTestId("room-template-research")).toContainText(
        "A cited answer with uncertainty made explicit.",
      );
      await expect(page.getByTestId("room-template-research")).toContainText(
        "The conclusion cites supporting entries",
      );
      await expect(page.getByTestId("room-template-research")).toContainText(
        "Example: assess a market or technical approach",
      );

      await page.getByLabel("Name").fill(roomTitle);
      await page.getByLabel("Objective").fill(customObjective);
      await page.getByTestId("room-template-planning").click();
      await expect(page.getByLabel("Objective")).toHaveValue(customObjective);
      await page.getByRole("button", { name: "Advanced configuration" }).click();
      await expect(page.getByLabel("Success criteria")).toHaveValue(
        /Dependencies and risks are explicit/,
      );
      await page.getByRole("combobox", { name: "Facilitator" }).click();
      await page.getByRole("option", { name: facilitatorName }).click();
      await page.getByRole("checkbox", { name: participantName }).click();
      await page.getByLabel("Schedule interval").click();
      await page.getByRole("option", { name: "1440 min" }).click();
      await expect(page.getByTestId("room-create-summary")).toContainText(
        `${facilitatorName} + 1 participants`,
      );
      await expect(page.getByTestId("room-create-summary")).toContainText(
        "Facilitator synthesis",
      );
      await expect(page.getByTestId("room-create-summary")).toContainText("8");
      await expect(page.getByTestId("room-create-summary")).toContainText("1440 min");
      await capture(page, testInfo, "template-first-create", 1440, 960);

      const responsePromise = page.waitForResponse(
        (response) =>
          response.url().endsWith("/api/rooms") &&
          response.request().method() === "POST",
      );
      await page.getByTestId("room-create-submit").click();
      const response = await responsePromise;
      expect(response.status()).toBe(201);
    });

    const roomResponse = await api.request("/api/rooms");
    expect(roomResponse.ok).toBe(true);
    const roomList = (await roomResponse.json()) as RoomDetailResponse["room"][];
    const createdRoom = roomList.find((room) => room.title === roomTitle);
    if (!createdRoom) throw new Error("Template-backed Room was not listed");
    expect(createdRoom.objective).toBe(customObjective);
    expect(createdRoom.schedule_interval_minutes).toBe(1440);
    const roomId = createdRoom.id;

    const seeded = await seedAcceptedOutcome(db, {
      workspaceId: workspace.id,
      roomId,
      agentId: facilitatorId,
      runtimeId,
      ownerId,
    });

    await test.step("open the exact outcome and inspect value signals", async () => {
      await page.goto(
        `${APP_URL}/${workspace.slug}/rooms?room=${roomId}&tab=outcome&focus=outcome_review&cycle_id=${seeded.cycleId}&memory_revision_id=${seeded.revisionId}`,
        { waitUntil: "domcontentloaded" },
      );
      await expect(page.getByTestId("room-detail")).toHaveAttribute("data-room-id", roomId);
      await expect(page.getByRole("tab", { name: "Outcome" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
      await expect(
        page.locator(`#room-outcome-revision-${seeded.revisionId}`),
      ).toBeFocused();
      await expect(page.getByTestId(`room-list-item-${roomId}`)).toContainText(
        "18 cost ticks",
      );
      await expect(page.getByText("Outcome review", { exact: true })).toBeVisible();
      await expect(page.getByTestId("room-wake")).toHaveAccessibleName("Run again");
      await capture(page, testInfo, "accepted-value-and-run-again", 1440, 960);
    });

    let duplicatedRoomId = "";
    await test.step("duplicate configuration only with schedules paused", async () => {
      await page.getByTestId("room-duplicate").click();
      const pausedCopyWarning = page.getByText(
        "This scheduled copy will be created paused",
      );
      await expect(pausedCopyWarning).toBeVisible();
      await page.setViewportSize({ width: 768, height: 900 });
      await pausedCopyWarning.scrollIntoViewIfNeeded();
      await capture(page, testInfo, "scheduled-duplicate-confirmation", 768, 900);
      const responsePromise = page.waitForResponse(
        (response) =>
          response.url().endsWith("/api/rooms") &&
          response.request().method() === "POST",
      );
      await page.getByTestId("room-create-submit").click();
      const response = await responsePromise;
      expect(response.status()).toBe(201);
      const duplicate = (await response.json()) as RoomDetailResponse;
      duplicatedRoomId = duplicate.room.id;
      expect(duplicate.room.status).toBe("paused");
      expect(duplicate.room.schedule_interval_minutes).toBe(1440);
      expect(duplicate.entries).toEqual([]);
      expect(duplicate.cycles).toEqual([]);
      expect(duplicate.turns).toEqual([]);
      expect(duplicate.memory_revisions).toEqual([]);
      expect(duplicate.recommendation_reviews).toEqual([]);
      expect(duplicate.artifacts).toEqual([]);
      await expect(page.getByTestId(`room-list-item-${duplicatedRoomId}`)).toContainText(
        "Weekly outcome review",
      );
      await page.setViewportSize({ width: 1440, height: 960 });
      await page.getByTestId(`room-list-item-${roomId}`).click();
      await expect(page.getByTestId("room-detail")).toHaveAttribute("data-room-id", roomId);
    });

    await test.step("Run again uses current capability instead of cached readiness", async () => {
      const preflight = await api.request(`/api/rooms/${roomId}/preflight?source=manual`);
      expect(preflight.ok).toBe(true);
      expect((await preflight.json()) as { allowed: boolean }).toMatchObject({ allowed: true });
      await expect(page.getByTestId("room-wake")).toHaveAccessibleName("Run again");

      await db.query(
        `UPDATE agent_runtime
         SET metadata = '{"capabilities":["room-tasks-v1"]}'::jsonb
         WHERE id = $1`,
        [runtimeId],
      );
      const beforeTasks = await db.query<{ count: number }>(
        `SELECT count(*)::int AS count
         FROM agent_task_queue task
         JOIN room_turn turn_row ON turn_row.id = task.room_turn_id
         WHERE turn_row.room_id = $1`,
        [roomId],
      );
      const refusedPromise = page.waitForResponse(
        (response) =>
          response.url().endsWith(`/api/rooms/${roomId}/wake`) &&
          response.request().method() === "POST",
      );
      await page.getByTestId("room-wake").click();
      expect((await refusedPromise).status()).toBe(409);
      const afterRefusalTasks = await db.query<{ count: number }>(
        `SELECT count(*)::int AS count
         FROM agent_task_queue task
         JOIN room_turn turn_row ON turn_row.id = task.room_turn_id
         WHERE turn_row.room_id = $1`,
        [roomId],
      );
      expect(afterRefusalTasks.rows[0]?.count).toBe(beforeTasks.rows[0]?.count);

      await db.query(
        `UPDATE agent_runtime
         SET metadata = '{"capabilities":["room-tasks-v1","room-outcomes-v2"]}'::jsonb,
             last_seen_at = now()
         WHERE id = $1`,
        [runtimeId],
      );
      await expect(page.getByTestId("room-wake")).toHaveAccessibleName("Run again");
      const acceptedPromise = page.waitForResponse(
        (response) =>
          response.url().endsWith(`/api/rooms/${roomId}/wake`) &&
          response.request().method() === "POST",
      );
      await page.getByTestId("room-wake").click();
      const accepted = await acceptedPromise;
      expect(accepted.status()).toBe(202);
      const payload = (await accepted.json()) as { tasks: string[] };
      expect(payload.tasks.length).toBeGreaterThan(0);
    });

    expect(duplicatedRoomId).not.toBe("");
    expect(browserFailures.pageErrors).toEqual([]);
    expect(browserFailures.requestFailures).toEqual([]);
    expect(browserFailures.consoleErrors).toEqual([]);
  } finally {
    if (workspaceId) await api.deleteWorkspace(workspaceId);
    await api.deleteUser();
    await db.end();
  }
});
