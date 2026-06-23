import { test, expect } from "./fixtures";

/**
 * Dashboard Extended tests cover:
 * - Header navigation icons
 * - Plug selector (tabs, add button)
 * - Add plug modal
 * - Speedometer gauge
 * - Stats panel cards
 * - Navigation to other pages
 *
 * Note: Settings modal tests are covered by Vitest unit tests
 * (page.test.tsx) because the SettingsModal returns null when !isOpen
 * in headless Chromium, making it impossible to trigger via E2E clicks.
 */
test.describe("Dashboard Extended", () => {
  test.beforeEach(async ({ page }) => {
    // Fixture already navigates to /dashboard. Just ensure page is ready.
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
  });

  test("should show the dashboard header with app title", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "EV Charge Controller" }),
    ).toBeVisible();
  });

  test("should show navigation icons in header", async ({ page }) => {
    await expect(page.getByRole("link", { name: /history/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /vehicles/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /settings/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /log out/i })).toBeVisible();
  });

  test("should show plug selector when plugs exist", async ({ page }) => {
    // Seed data has 2 plugs - at least one should be selected (pressed=true)
    const selectedPlug = page.getByRole("button", { pressed: true });
    await expect(selectedPlug.first()).toBeVisible({ timeout: 10_000 });
  });

  test("should show add vehicle option in settings for vehicle without plug", async ({
    page,
  }) => {
    // My RM2 has no charging plug — select it so Settings exposes the add option
    await expect(page.getByRole("button", { name: /My RM2/ })).toBeVisible({
      timeout: 10_000,
    });
    await page.getByRole("button", { name: /My RM2/ }).click();

    await page.getByRole("button", { name: "Open settings" }).click();
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toBeVisible({ timeout: 5_000 });
    await expect(
      dialog.getByRole("button", { name: /add charging plug/i }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
  });

  test("should open and close add vehicle modal", async ({ page }) => {
    await page.waitForLoadState("load");

    // Select My RM2 (no plug) so Settings exposes the "Add charging plug" action
    await expect(page.getByRole("button", { name: /My RM2/ })).toBeVisible({
      timeout: 10_000,
    });
    await page.getByRole("button", { name: /My RM2/ }).click();

    await page.getByRole("button", { name: "Open settings" }).click();
    const settingsDialog = page.locator("dialog[open]");
    await expect(settingsDialog).toBeVisible({ timeout: 5_000 });

    // "Add charging plug →" closes settings and opens AddPlugModal
    await settingsDialog
      .getByRole("button", { name: /add charging plug/i })
      .click();

    const addDialog = page.locator("dialog[open]");
    await expect(addDialog).toBeVisible({ timeout: 5_000 });
    await expect(
      addDialog.getByRole("heading", { name: /add vehicle/i }),
    ).toBeVisible();

    // Close via the Cancel button inside the dialog
    await addDialog.getByRole("button", { name: /cancel/i }).click();
    await expect(page.locator("dialog[open]")).toHaveCount(0, {
      timeout: 5_000,
    });
  });

  test("should show speedometer gauge", async ({ page }) => {
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });
  });

  test("should show gauge with slider role", async ({ page }) => {
    const gauge = page.getByRole("slider", { name: /speedometer/i });
    await expect(gauge).toBeVisible({ timeout: 10_000 });
  });

  test("should show stats panel cards", async ({ page }) => {
    // Use title attributes to disambiguate duplicate labels
    await expect(
      page
        .locator('span[title*="Time elapsed since charging started"]')
        .first(),
    ).toBeVisible({ timeout: 10_000 });
    await expect(
      page.locator('span[title*="Electricity stored in the battery"]').first(),
    ).toBeVisible();
    await expect(
      page.locator('span[title*="Electricity being drawn"]').first(),
    ).toBeVisible();
  });

  test("should navigate to history from dashboard header", async ({ page }) => {
    await page.getByRole("link", { name: /history/i }).click();
    await page.waitForURL(/\/history/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", { name: /charge history/i }),
    ).toBeVisible();
  });

  test("should navigate to vehicles from dashboard header", async ({
    page,
  }) => {
    await page.getByRole("link", { name: /vehicles/i }).click();
    await page.waitForURL(/\/vehicles/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();
  });
});
