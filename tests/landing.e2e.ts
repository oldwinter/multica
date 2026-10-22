import { test } from '@e2e-dev/web';
import { expect } from 'e2e';

// Smoke test for the public landing page. It renders without the API or
// database running, so this proves the Next.js app itself boots and serves
// the signed-out entry point.
test('landing page renders for signed-out visitors', async ({ app, browser }) => {
  await app.open('/');

  await expect(browser.locator('h1').first()).toContainText('Your next 10 hires');
  await expect(browser.locator('a[href="/login"]').first()).toBeVisible();
});
