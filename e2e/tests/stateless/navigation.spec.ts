import { test, expect } from "./fixtures";
import { VehiclesPage } from "../pages/vehiclesPage";
import { HistoryPage } from "../pages/historyPage";

/**
 * Navigation tests cover cross-page routing:
 * - Dashboard → History / Vehicles (header nav)
 * - Vehicles → Vehicle Detail → Vehicles (back)
 * - History → Dashboard (back)
 * - Vehicles → Dashboard (back)
 * - Cross-page navigation (History ↔ Vehicles)
 * - Browser back/forward navigation
 * - Direct URL access to each page
 */
test.describe("Navigation", () => {
  test("should navigate from dashboard to history page via header", async ({
    page,
  }) => {
    await page.goto("/dashboard");
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });

    await page.getByRole("link", { name: /history/i }).click();
    await page.waitForURL(/\/history/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { name: /charge history/i }),
    ).toBeVisible();
  });

  test("should navigate from dashboard to vehicles page via header", async ({
    page,
  }) => {
    await page.goto("/dashboard");
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });

    await page.getByRole("link", { name: /vehicles/i }).click();
    await page.waitForURL(/\/vehicles/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();
  });

  test("should navigate from vehicles to vehicle detail page", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const vehicleCount = await vehiclesPage.getVehicleCount();
    expect(vehicleCount).toBeGreaterThan(0);

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();

    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });
    expect(page.url()).toMatch(/\/vehicles\/[^/]+$/);
  });

  test("should navigate from vehicle detail back to vehicles list", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const vehicleCount = await vehiclesPage.getVehicleCount();
    expect(vehicleCount).toBeGreaterThan(0);

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    // Navigate back to vehicles list
    await page.getByRole("link", { name: /back to vehicles/i }).click();
    await page.waitForURL(/\/vehicles/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();
  });

  test("should navigate from history back to dashboard", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(
      page.getByRole("heading", { name: /charge history/i }),
    ).toBeVisible();

    await historyPage.goToDashboard();
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });
  });

  test("should navigate from vehicles back to dashboard", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();

    await vehiclesPage.goToDashboard();
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });
  });

  test("should navigate from history to vehicles via direct URL", async ({
    page,
  }) => {
    await page.goto("/history");
    await page.waitForURL(/\/history/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { name: /charge history/i }),
    ).toBeVisible();

    // Navigate to vehicles via direct URL (no header nav on history page)
    await page.goto("/vehicles");
    await page.waitForURL(/\/vehicles/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();
  });

  test("should navigate from vehicles to history via direct URL", async ({
    page,
  }) => {
    await page.goto("/vehicles");
    await page.waitForURL(/\/vehicles/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();

    // Navigate to history via direct URL (no header nav on vehicles page)
    await page.goto("/history");
    await page.waitForURL(/\/history/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { name: /charge history/i }),
    ).toBeVisible();
  });

  test("should support browser back navigation", async ({ page }) => {
    await page.goto("/dashboard");
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });

    // Navigate to history
    await page.getByRole("link", { name: /history/i }).click();
    await page.waitForURL(/\/history/, { timeout: 15_000 });

    // Go back to dashboard
    await page.goBack();
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });
  });

  test("should support browser forward navigation", async ({ page }) => {
    await page.goto("/dashboard");
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });

    // Navigate to history
    await page.getByRole("link", { name: /history/i }).click();
    await page.waitForURL(/\/history/, { timeout: 15_000 });

    // Go back to dashboard
    await page.goBack();
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });

    // Go forward to history
    await page.goForward();
    await page.waitForURL(/\/history/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", { name: /charge history/i }),
    ).toBeVisible();
  });

  test("should access dashboard directly via URL", async ({ page }) => {
    await page.goto("/dashboard");
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });

    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });
  });

  test("should access history directly via URL", async ({ page }) => {
    await page.goto("/history");
    await page.waitForURL(/\/history/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { name: /charge history/i }),
    ).toBeVisible();
  });

  test("should access vehicles directly via URL", async ({ page }) => {
    await page.goto("/vehicles");
    await page.waitForURL(/\/vehicles/, { timeout: 15_000 });

    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();
  });
});
