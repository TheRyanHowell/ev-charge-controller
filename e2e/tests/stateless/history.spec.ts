import { test, expect } from "./fixtures";
import { HistoryPage } from "../pages/historyPage";

test.describe("History Page", () => {
  test("should navigate to history page from dashboard", async ({ page }) => {
    await page.goto("/dashboard");
    await page.getByRole("link", { name: /history/i }).click();
    await page.waitForURL(/\/history/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", { name: /charge history/i }),
    ).toBeVisible();
  });

  test("should show session cards when sessions exist", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    // Seed always emits ≥2 sessions per vehicle (×2 vehicles) dated today.
    await expect(historyPage.sessionCards.first()).toBeVisible();
    const cardCount = await historyPage.getSessionCount();
    expect(cardCount).toBeGreaterThan(0);
  });

  test("should display vehicle filter dropdown", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.vehicleFilter).toBeVisible();
  });

  test("should display date picker", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.datePicker).toBeVisible();
  });

  test("should show vehicle filter options", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    // Default should be "All Vehicles"
    const selectedValue = await historyPage.vehicleFilter.inputValue();
    expect(selectedValue).toBe("all");
  });

  test("should expand and collapse session card", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    // Initially collapsed
    const wasExpanded = await historyPage.isSessionExpanded(0);
    expect(wasExpanded).toBe(false);

    // Expand first card - wait for aria-expanded to change
    await historyPage.toggleSessionCard(0);
    await page.waitForFunction(
      () => document.querySelector('[aria-expanded="true"]') !== null,
      {
        timeout: 5000,
      },
    );

    // Should now be expanded
    const isExpanded = await historyPage.isSessionExpanded(0);
    expect(isExpanded).toBe(true);

    // Collapse again - wait for aria-expanded to change back
    await historyPage.toggleSessionCard(0);
    await page.waitForFunction(
      () => document.querySelector('[aria-expanded="true"]') === null,
      {
        timeout: 5000,
      },
    );

    const isCollapsed = await historyPage.isSessionExpanded(0);
    expect(isCollapsed).toBe(false);
  });

  test("should navigate back to dashboard from history", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await historyPage.goToDashboard();
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });
  });

  test("should filter sessions by vehicle", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    const initialCount = await historyPage.getSessionCount();

    // Get available vehicle options
    const vehicleOptions = await historyPage.vehicleFilter
      .locator("option")
      .allTextContents();
    const vehicleNames = vehicleOptions.filter((opt) => opt !== "All Vehicles");

    if (vehicleNames.length > 0) {
      // Filter by first vehicle - wait for API to return filtered results
      const filterResponse = page.waitForResponse(
        (resp) => resp.url().includes("/api/history") && resp.status() === 200,
        { timeout: 15_000 },
      );
      await historyPage.selectVehicleFilter(vehicleNames[0]);
      await filterResponse;

      const filteredCount = await historyPage.getSessionCount();
      // Should have fewer or equal sessions when filtered
      expect(filteredCount).toBeLessThanOrEqual(initialCount);
    }
  });

  test("should show session status badges", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    // Status badges should be visible on session cards
    const statusBadges = page.locator(
      '[class*="rounded-md"][class*="text-xs"]',
    );
    const badgeCount = await statusBadges.count();
    expect(badgeCount).toBeGreaterThan(0);
  });

  test("should show energy added on session cards", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    // Energy added text should be visible (contains "kWh")
    const energyTexts = page.getByText(/kWh/i);
    const energyCount = await energyTexts.count();
    expect(energyCount).toBeGreaterThan(0);
  });
});
