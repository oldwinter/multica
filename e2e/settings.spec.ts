import { test, expect } from "@playwright/test";
import {
  authenticatePageWithApi,
  loginAsDefault,
  loginAsDefaultWithApi,
  waitForPageText,
} from "./helpers";

test.describe("Settings", () => {
  test("appearance follows the account, resolves offline conflicts, and boots without a default flash", async ({
    page,
    browser,
  }) => {
    test.setTimeout(360000);
    const { api, workspaceSlug } = await loginAsDefaultWithApi(page, {
      navigate: false,
    });
    const settingsUrl = `/${workspaceSlug}/settings?tab=preferences`;
    try {
      const hydrationErrors: string[] = [];
      page.on("console", (message) => {
        if (
          message.type() === "error" &&
          /hydration|did not match|server rendered/i.test(message.text())
        ) {
          hydrationErrors.push(message.text());
        }
      });
      page.on("pageerror", (error) => {
        if (/hydration|did not match|server rendered/i.test(error.message)) {
          hydrationErrors.push(error.message);
        }
      });

      await page.goto(settingsUrl, { waitUntil: "domcontentloaded" });
      await waitForPageText(page, "Appearance");
      await page.getByRole("radio", { name: /Relay/ }).click();
      await page.getByRole("radio", { name: "Dark" }).click();
      await expect(page.locator("html")).toHaveAttribute("data-skin", "relay");
      await expect(page.locator("html")).toHaveClass(/dark/);
      await expect(page.getByText("Synced across devices")).toBeVisible();

      const secondContext = await browser.newContext();
      const secondPage = await secondContext.newPage();
      try {
        await authenticatePageWithApi(secondPage, api);
        await secondPage.goto(
          new URL(
            `/${workspaceSlug}/settings?tab=preferences`,
            page.url(),
          ).toString(),
          { waitUntil: "domcontentloaded" },
        );
        await waitForPageText(secondPage, "Appearance", 60000);
        await expect(
          secondPage.getByRole("radio", { name: /Relay/ }),
        ).toBeChecked();
        await expect(
          secondPage.getByRole("radio", { name: "Dark" }),
        ).toBeChecked();

        await page.context().setOffline(true);
        await page.getByRole("radio", { name: /Field/ }).click();
        await expect(page.locator("html")).toHaveAttribute("data-skin", "field");
        await expect(page.getByText("Sync failed")).toBeVisible();

        // The second device writes later, so its server tuple must win when the
        // first device reconnects. The offline local choice remains visible
        // until that reconciliation completes.
        await secondPage.waitForTimeout(20);
        await secondPage.getByRole("radio", { name: /Tension/ }).click();
        await expect(
          secondPage.getByText("Synced across devices"),
        ).toBeVisible();
        await page.context().setOffline(false);
        await expect(page.locator("html")).toHaveAttribute(
          "data-skin",
          "tension",
        );
        await expect(page.getByText("Synced across devices")).toBeVisible();
      } finally {
        await secondContext.close();
      }

      await page.emulateMedia({ colorScheme: "dark" });
      await page.getByRole("radio", { name: "System" }).click();
      await expect(page.locator("html")).toHaveClass(/dark/);
      await page.emulateMedia({ colorScheme: "light" });
      await expect(page.locator("html")).not.toHaveClass(/dark/);

      // Use an explicit non-default tuple for the reload assertion, then record
      // every effective pre-hydration root mutation. The first valid state must
      // already be the cached choice; a transient default would be a user-visible
      // flash even if React later corrected it.
      await page.getByRole("radio", { name: /Relay/ }).click();
      await page.getByRole("radio", { name: "Dark" }).click();
      await expect(page.getByText("Synced across devices")).toBeVisible();
      await page.addInitScript(() => {
        const history: Array<{ skin: string | null; dark: boolean }> = [];
        const record = () => {
          const root = document.documentElement;
          if (!root) return;
          history.push({
            skin: root.dataset.skin ?? null,
            dark: root.classList.contains("dark"),
          });
        };
        Object.defineProperty(window, "__appearanceBootHistory", {
          value: history,
          configurable: true,
        });
        new MutationObserver(record).observe(document, {
          childList: true,
          subtree: true,
          attributes: true,
          attributeFilter: ["class", "data-skin", "style"],
        });
        record();
      });
      await page.reload({ waitUntil: "domcontentloaded" });
      await waitForPageText(page, "Appearance");
      const bootHistory = await page.evaluate(
        () =>
          (
            window as Window & {
              __appearanceBootHistory: Array<{
                skin: string | null;
                dark: boolean;
              }>;
            }
          ).__appearanceBootHistory,
      );
      const effectiveHistory = bootHistory.filter(
        (entry) => entry.skin !== null,
      );
      expect(effectiveHistory[0]).toEqual({ skin: "relay", dark: true });
      expect(effectiveHistory).not.toContainEqual({
        skin: "tension",
        dark: false,
      });
      expect(hydrationErrors).toEqual([]);

      await page.getByRole("button", { name: "Reset appearance" }).click();
      await expect(page.getByText(/skin to Tension/i)).toBeVisible();
      await page.getByRole("button", { name: "Reset to defaults" }).click();
      await expect(page.getByRole("radio", { name: /Tension/ })).toBeChecked();
      await expect(page.getByRole("radio", { name: "System" })).toBeChecked();
      await expect(page.getByText("Synced across devices")).toBeVisible();
    } finally {
      await page.context().setOffline(false).catch(() => undefined);
      await api.resetAppearancePreferences();
      await api.cleanup();
    }
  });

  test("appearance confidence and recovery", async ({ page, browser }) => {
    test.setTimeout(360000);
    const { api, workspaceSlug } = await loginAsDefaultWithApi(page, {
      navigate: false,
    });
    const settingsUrl = `/${workspaceSlug}/settings?tab=preferences`;
    let secondContext: Awaited<ReturnType<typeof browser.newContext>> | null =
      null;

    try {
      await api.resetAppearancePreferences();
      await page.goto(settingsUrl, { waitUntil: "domcontentloaded" });
      await waitForPageText(page, "Appearance", 60000);

      let appearancePatchCount = 0;
      page.on("request", (request) => {
        if (
          request.method() === "PATCH" &&
          new URL(request.url()).pathname === "/api/me" &&
          request.postData()?.includes('"skin"')
        ) {
          appearancePatchCount += 1;
        }
      });

      const previews = page.locator("[data-appearance-fixture]");
      await expect(previews).toHaveCount(3);
      await expect(
        previews.first().locator('[data-fixture-role="form-control"]'),
      ).toBeVisible();
      await expect(
        previews.first().locator('[data-fixture-role="code-editor"]'),
      ).toBeVisible();
      expect(appearancePatchCount).toBe(0);

      await page.getByRole("radio", { name: /Relay/ }).click();
      await expect(page.getByText("Synced across devices")).toBeVisible();
      await page.getByRole("button", { name: "Undo" }).click();
      await expect(
        page.getByText("Previous appearance restored"),
      ).toBeVisible();
      await expect(
        page.getByRole("radio", { name: /Tension/ }),
      ).toBeChecked();
      await expect(page.getByText("Synced across devices")).toBeVisible();
      expect(appearancePatchCount).toBe(2);

      secondContext = await browser.newContext();
      const secondPage = await secondContext.newPage();
      await authenticatePageWithApi(secondPage, api);
      await secondPage.goto(
        new URL(settingsUrl, page.url()).toString(),
        { waitUntil: "domcontentloaded" },
      );
      await waitForPageText(secondPage, "Appearance", 60000);

      await page.getByRole("radio", { name: /Field/ }).click();
      await expect(page.getByText("Synced across devices")).toBeVisible();
      await secondPage.waitForTimeout(25);
      await secondPage.getByRole("radio", { name: /Relay/ }).click();
      await expect(
        secondPage.getByText("Synced across devices"),
      ).toBeVisible();

      await page.getByRole("button", { name: "Undo" }).click();
      await expect(
        page.getByText(
          "Undo expired because appearance changed elsewhere",
        ),
      ).toBeVisible();
      await expect(page.getByRole("radio", { name: /Relay/ })).toBeChecked();

      await secondContext.close();
      secondContext = null;

      let failedAttempts = 0;
      await page.route("**/api/me", async (route) => {
        const request = route.request();
        if (
          request.method() === "PATCH" &&
          request.postData()?.includes('"skin":"field"')
        ) {
          failedAttempts += 1;
          await route.fulfill({
            status: 503,
            contentType: "application/json",
            body: JSON.stringify({ error: "appearance sync unavailable" }),
          });
          return;
        }
        await route.fallback();
      });

      await page.getByRole("radio", { name: /Field/ }).click();
      await expect(page.locator("html")).toHaveAttribute("data-skin", "field");
      await expect(page.getByText(/Sync failed/)).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Copy diagnostics" }),
      ).toHaveCount(0);
      await page.getByRole("button", { name: "Retry sync" }).click();
      await expect(
        page.getByRole("button", { name: "Copy diagnostics" }),
      ).toBeVisible();
      expect(failedAttempts).toBe(2);

      await page.context().grantPermissions(
        ["clipboard-read", "clipboard-write"],
        { origin: new URL(page.url()).origin },
      );
      await page.getByRole("button", { name: "Copy diagnostics" }).click();
      await expect(
        page.getByText("Appearance diagnostics copied"),
      ).toBeVisible();
      await page.unroute("**/api/me");

      await page.getByRole("button", { name: "Reset appearance" }).click();
      await expect(page.getByRole("alertdialog")).toBeVisible();
      await expect(page.getByText(/skin to Tension/i)).toBeVisible();
      await expect(page.getByText(/color mode to System/i)).toBeVisible();
      await expect(page.getByRole("radio", { name: /Field/ })).toBeChecked();
      await page.getByRole("button", { name: "Reset to defaults" }).click();
      await expect(
        page.getByRole("radio", { name: /Tension/ }),
      ).toBeChecked();
      await expect(page.getByRole("radio", { name: "System" })).toBeChecked();
      await expect(page.getByText("Synced across devices")).toBeVisible();
    } finally {
      await page.unroute("**/api/me").catch(() => undefined);
      if (secondContext) await secondContext.close();
      await api.resetAppearancePreferences();
      await api.cleanup();
    }
  });

  test("boots the active account skin when the shared projection belongs to another account", async ({
    page,
  }) => {
    test.setTimeout(180000);
    const { api, workspaceSlug } = await loginAsDefaultWithApi(page, {
      navigate: false,
    });
    try {
      await page.goto(`/${workspaceSlug}/settings?tab=preferences`, {
        waitUntil: "domcontentloaded",
      });
      await waitForPageText(page, "Appearance", 60000);
      await page.getByRole("radio", { name: /Relay/ }).click();
      await page.getByRole("radio", { name: "Dark" }).click();
      await expect(page.getByText("Synced across devices")).toBeVisible();

      await page.addInitScript(() => {
        const history: Array<{ skin: string | null; dark: boolean }> = [];
        const record = () => {
          const root = document.documentElement;
          if (!root) return;
          history.push({
            skin: root.dataset.skin ?? null,
            dark: root.classList.contains("dark"),
          });
        };
        Object.defineProperty(window, "__appearanceBootHistory", {
          value: history,
          configurable: true,
        });
        new MutationObserver(record).observe(document, {
          childList: true,
          subtree: true,
          attributes: true,
          attributeFilter: ["class", "data-skin", "style"],
        });
        record();
      });
      await page.evaluate(() => {
        const key = "multica-appearance-preferences";
        const raw = localStorage.getItem(key);
        if (!raw) throw new Error("appearance projection is missing");
        localStorage.setItem(
          key,
          JSON.stringify({
            ...(JSON.parse(raw) as Record<string, unknown>),
            skin: "field",
            requestedAppearance: "light",
            resolvedAppearance: "light",
          }),
        );
        localStorage.setItem(
          "multica-appearance-preferences-owner",
          "different-account",
        );
      });

      await page.reload({ waitUntil: "domcontentloaded" });
      await waitForPageText(page, "Appearance", 60000);
      const history = await page.evaluate(
        () =>
          (
            window as Window & {
              __appearanceBootHistory: Array<{
                skin: string | null;
                dark: boolean;
              }>;
            }
          ).__appearanceBootHistory.filter((entry) => entry.skin !== null),
      );
      expect(history[0]).toEqual({ skin: "relay", dark: true });
      expect(history).not.toContainEqual({ skin: "field", dark: false });
    } finally {
      await api.resetAppearancePreferences();
      await api.cleanup();
    }
  });

  test("updating workspace name reflects in sidebar immediately", async ({
    page,
  }) => {
    const workspaceSlug = await loginAsDefault(page);

    // Read the current workspace name from the sidebar
    const sidebarName = page.getByRole("button", { name: /E2E Workspace/ }).first();
    const originalName = (await sidebarName.innerText()).split("\n").pop()?.trim() ?? "E2E Workspace";

    await page.goto(`/${workspaceSlug}/settings?tab=workspace`, { waitUntil: "domcontentloaded" });
    await waitForPageText(page, "General");

    // Change workspace name
    const nameInput = page
      .locator('input[type="text"]')
      .first();
    await nameInput.clear();
    const newName = "Renamed WS " + Date.now();
    await nameInput.fill(newName);

    // Save
    await page.locator("button", { hasText: "Save" }).click();

    await expect(page.getByText("Workspace settings saved").first()).toBeVisible({ timeout: 5000 });

    // Sidebar should reflect the new name WITHOUT page refresh
    await expect(page.getByRole("button", { name: new RegExp(newName) }).first()).toBeVisible();

    // Restore original name so other tests aren't affected
    await nameInput.clear();
    await nameInput.fill(originalName.trim());
    await page.locator("button", { hasText: "Save" }).click();
    await expect(page.getByText("Workspace settings saved").first()).toBeVisible({ timeout: 5000 });
    await expect(page.getByRole("button", { name: new RegExp(originalName) }).first()).toBeVisible();
  });

  // Composio connect flow, fully mocked at the network boundary so it runs
  // without a configured COMPOSIO_API_KEY or a live Composio project. The
  // backend redirect is simulated by pointing the init endpoint's redirect_url
  // straight back at the settings page with ?connected=<slug> — exercising the
  // frontend's callback toast + connections refresh (MUL-3718) end to end.
  test("connecting a Composio toolkit shows a toast and refreshes the list", async ({
    page,
  }) => {
    const workspaceSlug = await loginAsDefault(page);
    const settingsUrl = `/${workspaceSlug}/settings?tab=integrations`;

    // Stateful: connections is empty until the (mocked) connect flow lands.
    let connected = false;

    await page.route("**/api/integrations/composio/toolkits", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          { slug: "notion", name: "Notion", connectable: true },
        ]),
      }),
    );

    await page.route("**/api/integrations/composio/connections", (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          connected
            ? [
                {
                  id: "conn-notion-1",
                  toolkit_slug: "notion",
                  status: "active",
                  connected_at: new Date().toISOString(),
                  last_used_at: null,
                },
              ]
            : [],
        ),
      });
    });

    await page.route("**/api/integrations/composio/connect/init", (route) => {
      // Composio would 302 through its hosted consent and back to our callback,
      // which emits CallbackRedirect's slug-less shape:
      // `/settings?tab=integrations&connected=<slug>`. The web proxy's
      // legacy-route redirect then prepends the last workspace slug, landing on
      // the real settings route. Mock that exact backend shape (NOT the final
      // slugged URL) so the test exercises the same redirect path real users hit.
      connected = true;
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          redirect_url: `/settings?tab=integrations&connected=notion`,
        }),
      });
    });

    await page.goto(settingsUrl, { waitUntil: "domcontentloaded" });
    await waitForPageText(page, "Composio");

    // Notion starts disconnected → click Connect.
    await page.getByRole("button", { name: /^Connect$/ }).first().click();

    // Success toast from the simulated callback redirect.
    await expect(page.getByText("Connected").first()).toBeVisible({ timeout: 10000 });

    // List refreshed without a manual reload: the Notion card now offers
    // Disconnect, and the one-shot ?connected param has been stripped.
    await expect(
      page.getByRole("button", { name: /Disconnect/ }).first(),
    ).toBeVisible({ timeout: 10000 });
    await expect(page).not.toHaveURL(/connected=notion/);
  });
});
