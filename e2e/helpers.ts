import { expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const DEFAULT_E2E_NAME = "E2E User";
const E2E_WORKER = process.env.TEST_PARALLEL_INDEX ?? process.env.TEST_WORKER_INDEX ?? "0";
const E2E_RUN_ID = process.env.E2E_RUN_ID ?? `${Date.now().toString(36)}-${process.pid.toString(36)}`;
const DEFAULT_E2E_EMAIL = `e2e-${E2E_WORKER}-${E2E_RUN_ID}@multica.ai`;
const DEFAULT_E2E_WORKSPACE = `e2e-workspace-${E2E_WORKER}-${E2E_RUN_ID}`;

async function waitForIssuesPage(page: Page) {
  await waitForPageText(page, "New Issue");
  await expect(page.getByRole("button", { name: "New Issue" })).toBeVisible({
    timeout: 15000,
  });
}

export async function waitForPageText(page: Page, text: string, timeout = 30000) {
  await page.waitForFunction(
    (expected) => document.body?.innerText.includes(expected),
    text,
    { timeout },
  );
}

export async function reloadAppPage(page: Page) {
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitForPageText(page, "Issues");
}

/**
 * Log in as the default E2E user and ensure the workspace exists first.
 * Authenticates via API (send-code → DB read → verify-code), then injects
 * the token into localStorage so the browser session is authenticated.
 *
 * Returns the E2E workspace slug so callers can build workspace-scoped URLs.
 */
export async function loginAsDefaultWithApi(
  page: Page,
  options: { navigate?: boolean } = {},
): Promise<{ api: TestApiClient; workspaceSlug: string }> {
  const api = new TestApiClient();
  await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
  const workspace = await api.ensureWorkspace(
    `E2E Workspace ${E2E_WORKER}`,
    DEFAULT_E2E_WORKSPACE,
  );
  await api.markUserOnboarded();

  await authenticatePageWithApi(page, api);
  if (options.navigate !== false) {
    await page.goto(`/${workspace.slug}/issues`, {
      waitUntil: "domcontentloaded",
    });
    await waitForIssuesPage(page);
  }
  return { api, workspaceSlug: workspace.slug };
}

export async function authenticatePageWithApi(
  page: Page,
  api: TestApiClient,
): Promise<void> {
  const token = api.getToken();
  if (!token) {
    throw new Error("E2E login did not return an auth token");
  }

  await page.addInitScript((t) => {
    localStorage.setItem("multica_token", t);
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
}

export async function loginAsDefault(
  page: Page,
  options: { navigate?: boolean } = {},
): Promise<string> {
  const { workspaceSlug } = await loginAsDefaultWithApi(page, options);
  return workspaceSlug;
}

/**
 * Create a TestApiClient logged in as the default E2E user.
 * Call api.cleanup() in afterEach to remove test data created during the test.
 */
export async function createTestApi(): Promise<TestApiClient> {
  const api = new TestApiClient();
  await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
  await api.ensureWorkspace(`E2E Workspace ${E2E_WORKER}`, DEFAULT_E2E_WORKSPACE);
  await api.markUserOnboarded();
  return api;
}

export async function preferManualCreateMode(page: Page) {
  await page.evaluate(() => {
    localStorage.setItem(
      "multica_create_mode",
      JSON.stringify({ state: { lastMode: "manual" }, version: 0 }),
    );
  });
  await reloadAppPage(page);
  await waitForIssuesPage(page);
}

export async function openWorkspaceMenu(page: Page) {
  // Click the workspace switcher button (has ChevronDown icon)
  const workspaceButton = page.getByRole("button", { name: /E2E Workspace/ }).first();
  await expect(workspaceButton).toBeVisible({ timeout: 15000 });
  await workspaceButton.click();
  // Wait for dropdown to appear
  await expect(page.locator('[class*="popover"]')).toBeVisible();
}
