import { test, expect } from "./fixtures";

/**
 * Logout tests cover:
 * - Logout button presence in dashboard header
 * - Logout flow (click button -> redirect to login)
 * - Post-logout state (unauthenticated, redirected from dashboard)
 */
test.describe("Logout", () => {
  test.beforeEach(async ({ page }) => {
    // Start from authenticated dashboard
    await page.goto("/dashboard");
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });
  });

  test("should show logout button in dashboard header", async ({ page }) => {
    const logoutButton = page.getByRole("button", { name: /log [oO]ut/i });
    await expect(logoutButton).toBeVisible();
  });

  test("should redirect to login page after clicking logout", async ({
    page,
  }) => {
    await page.getByRole("button", { name: /log [oO]ut/i }).click();

    // Should redirect to login page
    await page.waitForURL("/login", { timeout: 15_000 });
    await expect(page).toHaveURL("/login");
  });

  test("should show login form after logout", async ({ page }) => {
    await page.getByRole("button", { name: /log [oO]ut/i }).click();
    await page.waitForURL("/login", { timeout: 15_000 });

    // Login form should be visible
    await expect(page.locator("#email")).toBeVisible();
    await expect(page.locator("#password")).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  });

  test("should be unauthenticated after logout (dashboard redirects to login)", async ({
    page,
  }) => {
    // Perform logout
    await page.getByRole("button", { name: /log [oO]ut/i }).click();
    await page.waitForURL("/login", { timeout: 15_000 });

    // Try to access dashboard - should redirect to login
    await page.goto("/dashboard");
    await page.waitForURL("/login", { timeout: 15_000 });
    await expect(page).toHaveURL("/login");
  });

  test("should navigate via /logout page endpoint", async ({ page }) => {
    // Navigate directly to /logout
    await page.goto("/logout");

    // Should end up on login page
    await page.waitForURL("/login", { timeout: 15_000 });
    await expect(page).toHaveURL("/login");
  });
});
