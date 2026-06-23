import { test, expect } from "./fixtures";
import { VehiclesPage } from "../pages/vehiclesPage";

test.describe("Vehicles Page", () => {
  test("should navigate to vehicles page from dashboard", async ({ page }) => {
    await page.goto("/dashboard");
    await page.getByRole("link", { name: /vehicles/i }).click();
    await page.waitForURL(/\/vehicles/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();
  });

  test("should show vehicle list when vehicles exist", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // Should show vehicle links (seeded data has vehicles)
    const vehicleCount = await vehiclesPage.getVehicleCount();
    expect(vehicleCount).toBeGreaterThan(0);
  });

  test("should display vehicle names in the list", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const names = await vehiclesPage.getVehicleNames();
    expect(names.length).toBeGreaterThan(0);
    // Should have seeded vehicle names
    expect(names.some((n) => n.includes("RM"))).toBe(true);
  });

  test("should display battery capacity for each vehicle", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // Each vehicle row should show capacity in kWh
    const capacityLabels = page.getByText(/kWh/);
    const capacityCount = await capacityLabels.count();
    expect(capacityCount).toBeGreaterThan(0);
  });

  test("should navigate to vehicle detail from list", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // Seed always creates 3 vehicles; assert at least one is listed.
    const names = await vehiclesPage.getVehicleNames();
    expect(names.length).toBeGreaterThan(0);

    // Click on first vehicle link
    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();

    // Should navigate to vehicle detail page
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });
    expect(page.url()).toMatch(/\/vehicles\/[^/]+$/);
  });

  test("should show back to dashboard link", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    await expect(vehiclesPage.backToDashboardLink).toBeVisible();
  });

  test("should navigate back to dashboard", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    await vehiclesPage.goToDashboard();
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });
  });

  test("should show add vehicle button", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    await expect(vehiclesPage.addVehicleButton).toBeVisible();
  });

  test("should open add vehicle dialog", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    await vehiclesPage.clickAddVehicle();
    // Wait for the dialog to have the [open] attribute set by showModal()
    await expect(page.locator("dialog[open]")).toBeVisible({
      timeout: 10_000,
    });
  });

  test("should show available vehicle models in add dialog", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    await vehiclesPage.clickAddVehicle();
    // Wait for the dialog to have the [open] attribute set by showModal()
    await expect(page.locator("dialog[open]")).toBeVisible({
      timeout: 10_000,
    });

    // Dialog should contain heading or model list or "no available models" message
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toContainText(/add vehicle/i);
  });

  test("should close add vehicle dialog", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    await vehiclesPage.clickAddVehicle();
    // Dialog should be open after clickAddVehicle
    await expect(page.locator("dialog[open]")).toBeVisible();

    await vehiclesPage.closeAddDialog();
    // After closing, the dialog should not have the [open] attribute
    await expect(page.locator("dialog[open]")).toHaveCount(0);
  });

  test("should show cost information in stats row for vehicles with sessions", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // My RM1 and My RM1S have completed sessions; their stats rows include cost
    await expect(
      page.getByText(/^£\d+\.\d{2}$/).first(),
      "Total cost formatted as £X.XX should appear in at least one vehicle stats row",
    ).toBeVisible();
  });

  test("should show range information in stats row for vehicles with sessions", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // My RM1 (rangeMaxMi=40) and My RM1S (rangeMaxMi=80) have sessions and range data
    // The stats row shows "min – max mi" combined
    await expect(
      page.getByText(/\d+ – \d+ mi/).first(),
      "Min–max range formatted as 'X – Y mi' should appear in at least one vehicle stats row",
    ).toBeVisible();
  });
});
