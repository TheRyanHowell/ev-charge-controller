import { test as base, expect } from "@playwright/test";
import { ApiHelper, createApiHelper, resetAllState } from "../helpers/auth";

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? "http://localhost:3000";

/**
 * Stateful fixture: resets DB at the start of each TEST FILE.
 *
 * Tests that modify shared database state (charge sessions, vehicle percents,
 * plug assignments) must use this fixture and be wrapped in
 * `test.describe.serial()` to guarantee they execute serially on a single
 * worker. Each file's beforeEach must stop all sessions for all vehicles
 * as a defensive measure against parallel test files that may have left
 * state behind.
 */
export const test = base.extend<{ api: ApiHelper }>({
  // Page fixture: reset DB before each test.
  // Auth is restored from storageState (set by global-setup).
  page: async ({ page }, use) => {
    // Reset DB to seed state BEFORE proceeding with test.
    await resetAllState();

    // Navigate to dashboard to ensure fresh data from seed state.
    // Use 'load' to wait for all resources (including API responses).
    await page.goto(`${BASE_URL}/dashboard`, {
      waitUntil: "load",
    });

    // Wait for the gauge to be visible (ensures page is fully loaded
    // with fresh data from seed state).
    // Use extended timeout: after DB reset, API may need time to respond.
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 30_000,
    });

    await use(page);
  },

  // Optional API helper for tests that need direct backend access.
  api: async ({}, use) => {
    const helper = await createApiHelper();
    await use(helper);
  },
});

export { expect } from "@playwright/test";
