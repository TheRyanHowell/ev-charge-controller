import { test as base } from "@playwright/test";
import { ApiHelper, createApiHelper } from "../helpers/auth";

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? "http://localhost:3000";

/**
 * Stateless fixture: no per-test DB reset, no gauge wait.
 *
 * Database is reset once in global-setup. Tests that use this fixture
 * must NOT modify shared database state. They handle their own navigation
 * and readiness checks in beforeEach hooks.
 */
export const test = base.extend<{ api: ApiHelper }>({
  // Page fixture: navigate to dashboard.
  // Auth is restored from storageState (set by global-setup).
  // Tests do their own readiness checks in beforeEach hooks.
  page: async ({ page }, use) => {
    // Navigate to dashboard (may redirect to /login for isolated contexts).
    // Use 'domcontentloaded' because the dashboard polls every 5s,
    // so 'networkidle' would never resolve (requires 500ms of no requests).
    await page.goto(`${BASE_URL}/dashboard`, {
      waitUntil: "domcontentloaded",
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
