import { test, expect } from "./fixtures";

test.describe("E2E Infrastructure Smoke Tests", () => {
  test("should load the application", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page).toHaveTitle(/EV Charge/i);
  });

  test("should show login page when not authenticated", async ({ page }) => {
    await page.context().clearCookies();
    await page.goto("/login");
    await expect(page).toHaveURL(/\/login/);
  });

  test("should access dashboard after authentication", async ({ page }) => {
    // Tests start authenticated via storage state
    await page.goto("/dashboard");
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });
  });

  test("should show dashboard content after authentication", async ({
    page,
  }) => {
    // Tests start authenticated via storage state
    await page.goto("/dashboard");
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });
  });

  test("should navigate to history page after authentication", async ({
    page,
  }) => {
    // Tests start authenticated via storage state
    await page.goto("/dashboard");
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });
    // Wait for header nav to be rendered
    await page.getByRole("link", { name: /history/i }).waitFor({
      state: "visible",
      timeout: 10_000,
    });
    await page.getByRole("link", { name: /history/i }).click();
    await page.waitForURL(/\/history/, { timeout: 10_000 });
  });

  test("should reject invalid credentials via form", async ({ page }) => {
    await page.context().clearCookies();
    await page.goto("/login");
    await page.locator("#email").waitFor({ state: "visible" });

    await page.locator("#email").fill("wrong@example.com");
    await page.locator("#password").fill("wrongpassword");

    const [response] = await Promise.all([
      page.waitForResponse((resp) => resp.url().includes("/api/auth/login"), {
        timeout: 10_000,
      }),
      page.getByRole("button", { name: "Sign in" }).click(),
    ]);

    expect(response.status()).toBe(401);
  });
});
