import { test, expect } from "./fixtures";

/**
 * Dashboard States tests are read-only (no session manipulation).
 * They verify UI elements like header, navigation, gauge, and stats panel.
 * No beforeEach cleanup needed - these tests are safe to run in parallel
 * with stateful tests.
 */
test.describe("Dashboard States", () => {
  test("should show dashboard header with app title", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });

    // Header should show navigation links
    await expect(page.getByRole("link", { name: /history/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /vehicles/i })).toBeVisible();
  });

  test("should show navigation icons in header", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });

    // History icon
    await expect(
      page.locator('[aria-label="View charge history"]'),
    ).toBeVisible();
    // Vehicles icon
    await expect(page.locator('[aria-label="View vehicles"]')).toBeVisible();
  });

  test("should show vehicle selector when vehicles exist", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });

    // Vehicle chip buttons carry aria-pressed; filter to the exact text to avoid
    // matching StatusBar (which also shows the selected vehicle name) and My RM1S.
    await expect(
      page.locator("button[aria-pressed]").filter({ hasText: /^My RM1$/ }),
    ).toBeVisible({ timeout: 10_000 });
  });

  test("should show add vehicle option in settings for vehicle without plug", async ({
    page,
  }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });

    // My RM2 has no charging plug assigned — select it so Settings shows the add option
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
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });

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

    await page.keyboard.press("Escape");
    await expect(page.locator("dialog[open]")).toHaveCount(0, {
      timeout: 5_000,
    });
  });

  test("should show speedometer gauge", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });
  });

  test("should show gauge with slider role", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });

    // The gauge should have a slider for accessibility
    const slider = page.getByRole("slider");
    await expect(slider).toBeVisible();
  });

  test("should show stats panel with power and energy labels", async ({
    page,
  }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });

    // Stats panel should show labels like "Power", "Energy", "Time"
    const powerLabels = page.getByText(/power/i);
    expect(await powerLabels.count()).toBeGreaterThan(0);
    const energyLabels = page.getByText(/energy/i);
    expect(await energyLabels.count()).toBeGreaterThan(0);
  });

  test("should navigate to history from dashboard header", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });

    await page.getByRole("link", { name: /history/i }).click();
    await page.waitForURL(/\/history/, { timeout: 15000 });
    await expect(
      page.getByRole("heading", { name: /charge history/i }),
    ).toBeVisible();
  });

  test("should navigate to vehicles from dashboard header", async ({
    page,
  }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15000,
    });

    await page.getByRole("link", { name: /vehicles/i }).click();
    await page.waitForURL(/\/vehicles/, { timeout: 15000 });
    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();
  });
});
