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

interface RoomDetailResponse {
  room: { id: string; memory_version: number; status: string };
  entries: Array<{ id: string; type: string; body: string }>;
  cycles: Array<{ id: string; status: string; refusal_reason: string | null }>;
  artifacts: Array<{ id: string; kind: string; title: string }>;
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

async function capture(page: Page, testInfo: TestInfo, label: string, width: number, height: number) {
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

test("runs a durable Room from paused message through result promotion", async ({ page }, testInfo) => {
  test.setTimeout(240_000);
  const runId = `${Date.now().toString(36)}-${process.pid.toString(36)}-${testInfo.workerIndex}`;
  const email = `room-owner-${runId}@multica.ai`;
  const workspaceSlug = `room-e2e-${runId}`;
  const roomTitle = `Research council ${runId}`;
  const agentName = `Room analyst ${runId}`;
  const unsentDraft = `Unsent Room draft ${runId}`;
  const pausedMessage = `Captured while paused ${runId}`;
  const activeMessage = `Investigate the durable Room boundary ${runId}`;
  const completedOutput = `Decision: keep Room orchestration isolated ${runId}.`;
  const artifactTitle = `Room boundary decision ${runId}`;
  const api = new TestApiClient();
  const db = new pg.Client(DATABASE_URL);
  const signals = collectSignals(page);
  const actions: string[] = [];
  let workspaceId = "";
  let roomId = "";
  let resultEntryId = "";
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

    const runtimeResult = await db.query<{ id: string }>(
      `INSERT INTO agent_runtime (
         workspace_id, owner_id, name, runtime_mode, provider, status,
         visibility, device_info, metadata, last_seen_at
       ) VALUES ($1, $2, $3, 'cloud', 'room_e2e', 'online',
                 'private', 'Room E2E', '{}'::jsonb, now())
       RETURNING id::text`,
      [workspace.id, userId, `Room runtime ${runId}`],
    );
    const runtimeId = runtimeResult.rows[0]?.id;
    if (!runtimeId) throw new Error("Room E2E runtime was not created");

    await db.query(
      `INSERT INTO agent (
         workspace_id, name, description, instructions, runtime_mode,
         runtime_config, runtime_id, visibility, max_concurrent_tasks, owner_id
       ) VALUES ($1, $2, 'Room E2E analyst', '', 'cloud',
                 '{}'::jsonb, $3, 'workspace', 1, $4)`,
      [workspace.id, agentName, runtimeId, userId],
    );

    const token = api.getToken();
    if (!token) throw new Error("Room E2E login did not return a token");
    await authenticate(page, token);
    await page.setViewportSize({ width: 1440, height: 960 });
    await page.goto(`${APP_URL}/${workspace.slug}/rooms`, { waitUntil: "domcontentloaded" });
    await expect(page.locator("[data-room-workspace]")).toBeVisible({ timeout: 30_000 });

    await page.getByTestId("room-create-open").click();
    await page.getByLabel("Name").fill(roomTitle);
    await page.getByLabel("Instructions").fill("Compare evidence and preserve durable decisions.");
    await page.getByText("Select an agent", { exact: true }).click();
    await page.getByRole("option", { name: agentName }).click();
    const createResponsePromise = page.waitForResponse(
      (response) => response.url().endsWith("/api/rooms") && response.request().method() === "POST",
    );
    await page.getByTestId("room-create-submit").click();
    const createResponse = await createResponsePromise;
    expect(createResponse.status()).toBe(201);
    const created = (await createResponse.json()) as RoomDetailResponse;
    roomId = created.room.id;
    await expect(page.getByTestId(`room-list-item-${roomId}`)).toContainText(roomTitle);
    actions.push("created Room through shared UI");

    await expect(page.locator('[data-sonner-toast]')).toBeHidden({ timeout: 10_000 });
    await capture(page, testInfo, "room-starters-desktop", 1280, 800);
    await capture(page, testInfo, "room-starters-tablet", 768, 900);
    await capture(page, testInfo, "room-starters-mobile", 375, 812);
    const mobileStarterHeight = await page
      .getByTestId("room-starter-unblock")
      .evaluate((element) => element.getBoundingClientRect().height);
    expect(mobileStarterHeight).toBeGreaterThanOrEqual(44);
    await page.emulateMedia({ colorScheme: "dark" });
    await capture(page, testInfo, "room-starters-dark-desktop", 1280, 800);
    await page.emulateMedia({ colorScheme: "light" });
    await page.context().addCookies([
      { name: "multica-locale", value: "zh-Hans", url: APP_URL },
    ]);
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("room-starter-unblock")).toBeVisible({ timeout: 30_000 });
    await capture(page, testInfo, "room-starters-zh-mobile", 375, 812);
    await page.context().addCookies([
      { name: "multica-locale", value: "en", url: APP_URL },
    ]);
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("room-starter-unblock")).toBeVisible({ timeout: 30_000 });
    const starter = page.getByTestId("room-starter-unblock");
    await expect(page.locator('[data-testid^="room-starter-"]')).toHaveCount(3);
    await starter.click();
    await expect(page.getByTestId("room-message-input")).not.toHaveValue("");
    await expect(page.getByTestId("room-message-input")).toBeFocused();
    await expect(starter).toBeHidden();
    await page.getByTestId("room-message-input").fill("");
    await page.setViewportSize({ width: 1440, height: 960 });
    actions.push("prefilled a focused draft from a Room starter without posting");

    await test.step("restore an unsent Room draft after reload", async () => {
      const messageInput = page.getByTestId("room-message-input");
      await messageInput.fill(unsentDraft);
      await page.reload({ waitUntil: "domcontentloaded" });
      await expect(page.locator("[data-room-workspace]")).toBeVisible({ timeout: 30_000 });
      await expect(page.getByTestId("room-message-input")).toHaveValue(unsentDraft);
    });
    actions.push("restored unsent Room draft after reload");

    await test.step("pause the room and persist a refused message", async () => {
      const statusToggle = page.getByTestId("room-status-toggle");
      await expect(statusToggle).toBeVisible();
      await expect(statusToggle).toBeEnabled();
      const statusResponsePromise = page.waitForResponse(
        (response) =>
          response.url().endsWith(`/api/rooms/${roomId}/status`) &&
          response.request().method() === "PUT",
        { timeout: 15_000 },
      );
      await statusToggle.evaluate((element) => (element as HTMLButtonElement).click());
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
    const refused = (await pausedResponse.json()) as RoomDetailResponse & {
      cycle: { refusal_reason: string };
    };
    expect(refused.cycle.refusal_reason).toBe("room_paused");
    await expect(page.getByText(pausedMessage, { exact: true })).toBeVisible({ timeout: 15_000 });
    await expect(
      page.locator('[data-sonner-toast][data-type="warning"]').filter({
        hasText: "Message was saved, but execution did not start",
      }),
    ).toBeVisible();
    await capture(page, testInfo, "room-paused-warning", 1440, 960);
    await expect(page.getByTestId("room-wake")).toBeDisabled();
    const pausedCounts = await db.query<{ entries: number; tasks: number }>(
      `SELECT
         (SELECT count(*)::int FROM room_entry WHERE room_id = $1 AND body = $2) AS entries,
         (SELECT count(*)::int FROM agent_task_queue q
          JOIN room_turn t ON t.id = q.room_turn_id WHERE t.room_id = $1) AS tasks`,
      [roomId, pausedMessage],
    );
    expect(pausedCounts.rows[0]).toEqual({ entries: 1, tasks: 0 });
    actions.push("paused message persisted with refusal and no task");

    await test.step("resume the room", async () => {
      const statusToggle = page.getByTestId("room-status-toggle");
      const statusResponsePromise = page.waitForResponse(
        (response) =>
          response.url().endsWith(`/api/rooms/${roomId}/status`) &&
          response.request().method() === "PUT",
        { timeout: 15_000 },
      );
      await statusToggle.evaluate((element) => (element as HTMLButtonElement).click());
      expect((await statusResponsePromise).status()).toBe(200);
      await expect(page.getByText("Active", { exact: true }).first()).toBeVisible();
    });
    await page.getByRole("button", { name: "Notify agents" }).click();
    await page.getByRole("menuitemcheckbox", { name: agentName }).click();
    await page.keyboard.press("Escape");
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(page.getByTestId("room-mention-count")).toHaveText("1");
    await capture(page, testInfo, "room-mobile-mention", 390, 844);
    await page.setViewportSize({ width: 1440, height: 960 });
    const activeResponsePromise = page.waitForResponse(
      (response) =>
        response.url().endsWith(`/api/rooms/${roomId}/messages`) &&
        response.request().method() === "POST",
    );
    await page.getByTestId("room-message-input").fill(activeMessage);
    await page.getByTestId("room-message-send").click();
    const activeResponse = await activeResponsePromise;
    expect(activeResponse.status()).toBe(201);
    const activePayload = (await activeResponse.json()) as {
      cycle: { id: string; status: string };
      tasks: string[];
    };
    expect(activePayload.cycle.status).toBe("queued");
    expect(activePayload.tasks).toHaveLength(1);
    taskIds.push(...activePayload.tasks);
    actions.push("mentioned agent and queued exactly one Room task");

    await db.query(
      `UPDATE agent_task_queue
       SET status = 'completed', result = $2::jsonb, session_id = $3,
           work_dir = '/tmp/room-e2e', started_at = now(), completed_at = now()
       WHERE id = $1`,
      [activePayload.tasks[0], JSON.stringify({ output: completedOutput }), `room-e2e-${runId}`],
    );

    await expect
      .poll(
        async () => {
          const response = await api.request(`/api/rooms/${roomId}`);
          if (!response.ok) return 0;
          const detail = (await response.json()) as RoomDetailResponse;
          return detail.entries.filter((entry) => entry.type === "result").length;
        },
        { timeout: 90_000, intervals: [2_000, 5_000, 10_000] },
      )
      .toBe(1);
    const reconciledResponse = await api.request(`/api/rooms/${roomId}`);
    expect(reconciledResponse.ok).toBe(true);
    const reconciled = (await reconciledResponse.json()) as RoomDetailResponse;
    resultEntryId = reconciled.entries.find((entry) => entry.type === "result")?.id ?? "";
    expect(resultEntryId).not.toBe("");
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId(`room-entry-${resultEntryId}`)).toContainText(completedOutput, {
      timeout: 30_000,
    });
    await expect(page.getByText("Version 1", { exact: true })).toBeVisible();
    actions.push("scheduler reconciliation projected result and memory");

    await page.getByTestId(`room-promote-entry-${resultEntryId}`).click();
    await page.getByRole("combobox").click();
    await page.getByRole("option", { name: "Decision" }).click();
    await page.getByLabel("Title").fill(artifactTitle);
    const promoteResponsePromise = page.waitForResponse(
      (response) =>
        response.url().endsWith(`/api/rooms/${roomId}/promotions`) &&
        response.request().method() === "POST",
    );
    await page.getByTestId("room-promote-submit").click();
    const promoteResponse = await promoteResponsePromise;
    expect(promoteResponse.status()).toBe(201);
    await expect(page.getByText(artifactTitle, { exact: true })).toBeVisible({ timeout: 15_000 });
    actions.push("promoted result to immutable Room decision");

    const responsive = [
      await capture(page, testInfo, "room-desktop", 1440, 960),
      await capture(page, testInfo, "room-mobile", 390, 844),
    ];
    const compactMobileList = await page.getByTestId("room-list").boundingBox();
    expect(compactMobileList?.height).toBeLessThanOrEqual(160);

    await test.step("surface activity that arrives while reading history", async () => {
      await page.setViewportSize({ width: 1440, height: 960 });
      const statusToggle = page.getByTestId("room-status-toggle");
      const pauseResponsePromise = page.waitForResponse(
        (response) =>
          response.url().endsWith(`/api/rooms/${roomId}/status`) &&
          response.request().method() === "PUT",
      );
      await statusToggle.click();
      expect((await pauseResponsePromise).status()).toBe(200);

      const messageInput = page.getByTestId("room-message-input");
      const messageSend = page.getByTestId("room-message-send");
      for (let index = 0; index < 8; index += 1) {
        const responsePromise = page.waitForResponse(
          (response) =>
            response.url().endsWith(`/api/rooms/${roomId}/messages`) &&
            response.request().method() === "POST",
        );
        await messageInput.fill(`Background note ${index + 1} ${runId}`);
        await messageSend.click();
        expect((await responsePromise).status()).toBe(409);
        await expect(messageInput).toHaveValue("");
      }

      await expect(page.locator('[data-testid^="room-entry-"]')).toHaveCount(11, {
        timeout: 15_000,
      });
      const transcript = page.getByTestId("room-transcript");
      await transcript.evaluate((element) => {
        element.scrollTop = 0;
        element.dispatchEvent(new Event("scroll"));
      });

      const liveMessage = `New while reading history ${runId}`;
      const liveResponsePromise = page.waitForResponse(
        (response) =>
          response.url().endsWith(`/api/rooms/${roomId}/messages`) &&
          response.request().method() === "POST",
      );
      await messageInput.fill(liveMessage);
      await messageSend.click();
      expect((await liveResponsePromise).status()).toBe(409);

      const newEntries = page.getByTestId("room-transcript-new-entries");
      await expect(newEntries).toContainText("1 new");
      await expect(newEntries).toHaveAccessibleName("Show 1 new updates");
      responsive.push(
        await capture(page, testInfo, "room-unseen-activity-desktop", 1280, 800),
        await capture(page, testInfo, "room-unseen-activity-tablet", 768, 900),
        await capture(page, testInfo, "room-unseen-activity-mobile", 375, 812),
      );
      await newEntries.click();
      await expect(newEntries).toBeHidden();
      await expect(transcript).toBeFocused();
      const distanceFromLatest = await transcript.evaluate(
        (element) => element.scrollHeight - element.clientHeight - element.scrollTop,
      );
      expect(distanceFromLatest).toBeLessThanOrEqual(1);
      actions.push("surfaced unseen activity and returned to the live edge");
    });

    await test.step("keep longer Room lists within the mobile viewport budget", async () => {
      await page.setViewportSize({ width: 1440, height: 960 });
      for (let index = 1; index <= 5; index += 1) {
        await page.getByTestId("room-create-open").click();
        await page.getByLabel("Name").fill(`${roomTitle} ${index + 1}`);
        await page.getByText("Select an agent", { exact: true }).click();
        await page.getByRole("option", { name: agentName }).click();
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
      actions.push("long mobile Room list stayed capped and scrollable");
    });

    await page.setViewportSize({ width: 1440, height: 960 });
    await page.goto(`${APP_URL}/${workspace.slug}/issues`, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "New Issue" })).toBeVisible({ timeout: 30_000 });
    actions.push("existing Issue route remained operational");

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
          turns: number;
          tasks: number;
        }>(
          `SELECT
             (SELECT count(*)::int FROM room WHERE workspace_id = $1) AS rooms,
             (SELECT count(*)::int FROM room_entry WHERE workspace_id = $1) AS entries,
             (SELECT count(*)::int FROM room_turn WHERE workspace_id = $1) AS turns,
             (SELECT count(*)::int FROM agent_task_queue WHERE id = ANY($2::uuid[])) AS tasks`,
          [workspaceId, taskIds],
        );
        expect(cleanup.rows[0]).toEqual({ rooms: 0, entries: 0, turns: 0, tasks: 0 });
      });
    }
    await attempt(() => deleteTestUser(db, email));
    await attempt(() => db.end());

    if (testFailed) throw testError;
    if (cleanupErrors.length > 0) throw cleanupErrors[0];
  }
});
