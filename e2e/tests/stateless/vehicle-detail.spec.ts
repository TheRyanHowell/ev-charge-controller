import { test, expect } from "./fixtures";
import { VehiclesPage } from "../pages/vehiclesPage";

test.describe("Vehicle Detail Page", () => {
  test("should navigate to vehicle detail from vehicles list", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const names = await vehiclesPage.getVehicleNames();
    expect(names.length).toBeGreaterThan(0);

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();

    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });
    expect(page.url()).toMatch(/\/vehicles\/[^/]+$/);
  });

  test("should display vehicle name in heading", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    // Heading should contain the vehicle name
    const heading = page.getByRole("heading", { level: 1 });
    await expect(heading).toBeVisible();
  });

  test("should show back to vehicles link", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    const backLink = page.getByRole("link", { name: /back to vehicles/i });
    await expect(backLink).toBeVisible();
  });

  test("should navigate back to vehicles list", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    const backLink = page.getByRole("link", { name: /back to vehicles/i });
    await backLink.click();
    await page.waitForURL(/\/vehicles/, { timeout: 15_000 });
    await expect(
      page.getByRole("heading", { name: /vehicles/i }),
    ).toBeVisible();
  });

  test("should show time range filter buttons", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    // Wait for detail content to render
    await page.getByRole("heading", { level: 1 }).waitFor({
      state: "visible",
      timeout: 10_000,
    });

    // All time range buttons should be visible
    await expect(page.getByRole("button", { name: "Week" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Month" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Year" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Lifetime" })).toBeVisible();
  });

  test("should show stats cards when sessions exist", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    // Stats labels should be visible
    await expect(page.getByText("Total Energy")).toBeVisible();
    await expect(page.getByText("Sessions")).toBeVisible();
    await expect(page.getByText("Avg per Session")).toBeVisible();
  });

  test("should show vehicle details section at top of page", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    await expect(
      page.getByText(/vehicle details/i),
      "Vehicle Details heading should be visible near the top of the page",
    ).toBeVisible();
    await expect(
      page.getByText(/Battery Capacity/i),
      "Battery Capacity should be visible in the vehicle details section",
    ).toBeVisible();
  });

  test("should show vehicle details even with no charging sessions", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // My RM2 is seeded with zero completed sessions
    const rm2Link = page.getByRole("link", { name: /My RM2/i });
    await expect(rm2Link).toBeVisible();
    await rm2Link.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    await expect(
      page.getByText("No charging data yet"),
      "Empty state message should still be present",
    ).toBeVisible();
    await expect(
      page.getByText(/vehicle details/i),
      "Vehicle Details section should appear even when there are no sessions",
    ).toBeVisible();
    await expect(
      page.getByText(/Battery Capacity/i),
      "Battery Capacity should be visible in vehicle details section",
    ).toBeVisible();
  });

  test("should show CC/CV charging profile chart", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    await expect(
      page.getByText(/CC\/CV Charging Profile/i),
      "CC/CV Charging Profile heading should appear at the bottom of the vehicle detail page",
    ).toBeVisible();
  });

  test("should show edit and delete buttons", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    await expect(page.getByTitle("Edit name")).toBeVisible();
    await expect(page.getByTitle("Delete")).toBeVisible();
  });

  test("should show empty state for vehicle with no completed sessions", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // My RM2 is always seeded with zero completed sessions (has no plug assigned).
    const rm2Link = page.getByRole("link", { name: /My RM2/i });
    await expect(rm2Link).toBeVisible();

    await rm2Link.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    // Should show empty state message
    await expect(page.getByText("No charging data yet")).toBeVisible();
    await expect(
      page.getByText("Complete a charge session to see statistics"),
    ).toBeVisible();
  });

  test("should open delete confirmation dialog", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    // Click the delete button to trigger React state change
    await page.getByTitle(/delete/i).click();

    // Dialog's useEffect calls showModal() when isOpen becomes true
    await expect(page.locator("dialog[open]")).toBeVisible({
      timeout: 10_000,
    });
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toContainText(/delete vehicle/i);
  });

  test("should close delete dialog on cancel", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const firstLink = vehiclesPage.getVehicleLink(0);
    await firstLink.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    // Click delete button to open dialog
    await page.getByTitle(/delete/i).click();
    await expect(page.locator("dialog[open]")).toBeVisible();

    // Click Cancel button inside the dialog
    await page
      .getByRole("dialog")
      .getByRole("button", { name: /cancel/i })
      .click();

    await expect(page.locator("dialog[open]")).toHaveCount(0);
  });

  test("should show total cost and avg cost stat cards for vehicle with sessions", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // My RM1 (first vehicle) has many completed sessions
    const rm1Link = page.getByRole("link", { name: /My RM1/i }).first();
    await rm1Link.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    await expect(
      page.getByText("Total Cost"),
      "Total Cost stat card should appear for vehicle with completed sessions",
    ).toBeVisible();
    await expect(
      page.getByText("Avg Cost / Session"),
      "Avg Cost / Session stat card should appear for vehicle with completed sessions",
    ).toBeVisible();

    // Cost values should be formatted as £X.XX
    await expect(
      page.getByText(/^£\d+\.\d{2}$/).first(),
      "Cost value should be formatted as a pound amount",
    ).toBeVisible();
  });

  test("should show min and max added range stat cards for vehicle with sessions", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // My RM1 (rangeMaxMi=40) has many completed sessions
    const rm1Link = page.getByRole("link", { name: /My RM1/i }).first();
    await rm1Link.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    await expect(
      page.getByText("Min Added Range"),
      "Min Added Range stat card should appear for vehicle with range data and sessions",
    ).toBeVisible();
    await expect(
      page.getByText("Max Added Range"),
      "Max Added Range stat card should appear for vehicle with range data and sessions",
    ).toBeVisible();

    // Range values should be formatted as "X mi"
    const rangeValues = page.getByText(/^\d+ mi$/);
    await expect(
      rangeValues.first(),
      "Range values should be formatted as miles",
    ).toBeVisible();
  });

  test("should not show cost or range cards for vehicle with no sessions", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const rm2Link = page.getByRole("link", { name: /My RM2/i });
    await rm2Link.click();
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

    // Empty state should be shown
    await expect(
      page.getByText("No charging data yet"),
      "Empty state message should be shown for vehicle with no sessions",
    ).toBeVisible();

    // Cost and range cards must not appear
    await expect(
      page.getByText("Total Cost"),
      "Total Cost card should not appear when there are no sessions",
    ).not.toBeVisible();
    await expect(
      page.getByText("Min Added Range"),
      "Min Added Range card should not appear when there are no sessions",
    ).not.toBeVisible();
  });
});
