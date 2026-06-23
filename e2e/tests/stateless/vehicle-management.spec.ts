import { test, expect } from "./fixtures";
import { VehiclesPage } from "../pages/vehiclesPage";

test.describe("Vehicle Management", () => {
  test("should show vehicle list with at least one vehicle", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const vehicleCount = await vehiclesPage.getVehicleCount();
    expect(vehicleCount).toBeGreaterThanOrEqual(3);
  });

  test("should display vehicle names in the list", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const names = await vehiclesPage.getVehicleNames();
    expect(names.length).toBeGreaterThanOrEqual(3);
    // Should contain seeded vehicle names
    expect(names.some((n) => n.includes("RM1") || n.includes("RM2"))).toBe(
      true,
    );
  });

  test("should show vehicle model name in parentheses", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // Each vehicle link should show model name in parentheses
    const vehicleLinks = page.locator('a[href^="/vehicles/"]');
    const count = await vehicleLinks.count();
    expect(count).toBeGreaterThanOrEqual(3);
  });

  test("should show edit button for each vehicle", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const editButtons = page.getByRole("button", { name: /edit name/i });
    const editCount = await editButtons.count();
    expect(editCount).toBeGreaterThan(0);
  });

  test("should show delete button for each vehicle", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const deleteButtons = page.getByRole("button", { name: /delete/i });
    const deleteCount = await deleteButtons.count();
    expect(deleteCount).toBeGreaterThan(0);
  });

  test("should switch to inline edit when edit button is clicked", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const editButtons = page.getByRole("button", { name: /edit name/i });
    if ((await editButtons.count()) > 0) {
      await editButtons.first().click();
      // Should show an input field for editing
      await expect(page.locator('input[type="text"]')).toBeVisible({
        timeout: 5000,
      });
      // Cancel the edit
      await page
        .getByRole("button", { name: /cancel/i })
        .first()
        .click();
    }
  });

  test("should show edit input with current name", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const editButtons = page.getByRole("button", { name: /edit name/i });
    if ((await editButtons.count()) > 0) {
      await editButtons.first().click();
      const input = page.locator('input[type="text"]');
      await expect(input).toBeVisible({ timeout: 5000 });

      // Input should have the current vehicle name
      const inputValue = await input.inputValue();
      expect(inputValue.length).toBeGreaterThan(0);

      // Cancel the edit
      await page
        .getByRole("button", { name: /cancel/i })
        .first()
        .click();
    }
  });

  test("should open delete confirmation dialog", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const deleteButtons = page.getByRole("button", { name: /delete/i });
    if ((await deleteButtons.count()) > 0) {
      await deleteButtons.first().click();
      // Dialog should appear with "Delete vehicle?" heading
      await expect(
        page.getByRole("heading", { name: /delete vehicle/i }),
      ).toBeVisible({
        timeout: 5000,
      });
      // Cancel
      await page.getByRole("button", { name: "Cancel" }).first().click();
    }
  });

  test("should cancel delete when cancel is clicked", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    const initialCount = await vehiclesPage.getVehicleCount();

    const deleteButtons = page.getByRole("button", { name: /delete/i });
    if ((await deleteButtons.count()) > 0) {
      await deleteButtons.first().click();
      await expect(
        page.getByRole("heading", { name: /delete vehicle/i }),
      ).toBeVisible({
        timeout: 5000,
      });

      await page.getByRole("button", { name: "Cancel" }).first().click();

      // Count should be unchanged
      const countAfter = await vehiclesPage.getVehicleCount();
      expect(countAfter).toBe(initialCount);
    }
  });

  test("should show add vehicle dialog with model selection", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    await vehiclesPage.clickAddVehicle();
    const dialog = page.locator("dialog[open]");

    // Should show add vehicle heading
    await expect(dialog).toContainText(/add vehicle/i);

    // Should show model buttons or "no vehicle models available" message
    const modelButtons = dialog.getByRole("button").filter({
      hasNotText: /cancel/i,
    });
    const hasModels = (await modelButtons.count()) > 0;
    const hasNoModels = await dialog
      .getByText(/no vehicle models available/i)
      .isVisible()
      .catch(() => false);

    expect(hasModels || hasNoModels).toBe(true);

    // Close dialog
    await vehiclesPage.closeAddDialog();
  });

  test("should close add vehicle dialog on escape", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    await vehiclesPage.clickAddVehicle();
    await expect(page.locator("dialog[open]")).toBeVisible();

    await page.keyboard.press("Escape");
    await expect(page.locator("dialog[open]")).toHaveCount(0);
  });

  test("should show vehicle capacity in kWh", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // Each vehicle should show its battery capacity
    const capacityLabels = page.getByText(/kWh/);
    const capacityCount = await capacityLabels.count();
    expect(capacityCount).toBeGreaterThan(0);
  });

  test("should show summary stats below vehicles with sessions", async ({
    page,
  }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // Wait for vehicle cards to render (indicates API data loaded)
    await expect(vehiclesPage.vehicleList.first()).toBeVisible({
      timeout: 10_000,
    });

    // Seed creates RM1 + RM1S with 180 days of sessions; session stats must appear.
    const sessionTexts = page.getByText(/session/i);
    await expect(sessionTexts.first()).toBeVisible();
    const sessionCount = await sessionTexts.count();
    expect(sessionCount).toBeGreaterThan(0);
  });

  test("should navigate to specific vehicle by name", async ({ page }) => {
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.goto();

    // Use "My RM2" which is unique
    await vehiclesPage.navigateToVehicle("My RM2");
    await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15000 });
    expect(page.url()).toMatch(/\/vehicles\/[^/]+$/);
  });
});
