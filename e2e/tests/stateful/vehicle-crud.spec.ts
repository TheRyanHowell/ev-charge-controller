import { test, expect } from "./fixtures";
import { VehiclesPage } from "../pages/vehiclesPage";

/**
 * Stateful tests for vehicle creation, rename, and deletion.
 * Each test resets the DB to seed state (3 seeded vehicles).
 */
test.describe.serial("Vehicle CRUD", () => {
  test("should create a new vehicle from the vehicles page", async ({
    page,
  }) => {
    await page.goto("/vehicles");
    const vehiclesPage = new VehiclesPage(page);

    const initialCount = await vehiclesPage.getVehicleCount();
    expect(
      initialCount,
      "Seed should provide at least one vehicle",
    ).toBeGreaterThan(0);

    await vehiclesPage.clickAddVehicle();

    // Dialog must be open with model list
    const dialog = page.locator("dialog[open]");
    await expect(dialog, "Add vehicle dialog must open").toBeVisible({
      timeout: 5000,
    });
    await expect(dialog, "Dialog must contain model buttons").toContainText(
      /maeving/i,
    );

    // Select the first Maeving model (the battery-less Generic Vehicle is
    // listed first; generic creation is covered by generic-vehicle.spec.ts)
    const modelButton = dialog
      .getByRole("button")
      .filter({ hasText: /maeving/i })
      .first();
    await modelButton.click();

    // Dialog should close after selection
    await expect(
      page.locator("dialog[open]"),
      "Dialog must close after vehicle is created",
    ).toHaveCount(0, { timeout: 10_000 });

    // Vehicle list should now have one more entry
    await expect(
      page.locator('a[href^="/vehicles/"]'),
      "New vehicle must appear in the list",
    ).toHaveCount(initialCount + 1, { timeout: 10_000 });
  });

  test("should rename a vehicle inline", async ({ page }) => {
    await page.goto("/vehicles");
    const vehiclesPage = new VehiclesPage(page);

    // Start editing the first vehicle
    const editBtn = vehiclesPage.getEditButton(0);
    await editBtn.click();

    const input = page.locator('input[type="text"]').first();
    await expect(
      input,
      "Edit input must appear after clicking Edit Name",
    ).toBeVisible({ timeout: 5000 });

    // Clear and type a new name
    await input.fill("My Renamed Vehicle");
    await input.press("Enter");

    // Input should disappear (edit committed)
    await expect(
      page.locator('input[type="text"]'),
      "Edit input must disappear after committing",
    ).toHaveCount(0, { timeout: 5000 });

    // New name should appear in the list
    await expect(
      page.getByText("My Renamed Vehicle"),
      "Renamed vehicle must appear in the list",
    ).toBeVisible({ timeout: 5000 });
  });

  test("should delete a vehicle after confirmation", async ({ page }) => {
    await page.goto("/vehicles");
    const vehiclesPage = new VehiclesPage(page);

    const initialCount = await vehiclesPage.getVehicleCount();
    expect(initialCount, "Need at least one vehicle to delete").toBeGreaterThan(
      0,
    );

    // Click delete on the last vehicle (avoid deleting ones with active plugs)
    const deleteBtn = vehiclesPage.getDeleteButton(initialCount - 1);
    await deleteBtn.click();

    // Confirm delete
    await expect(
      page.getByRole("heading", { name: /delete vehicle/i }),
      "Delete confirmation dialog must appear",
    ).toBeVisible({ timeout: 5000 });

    await vehiclesPage.confirmDelete();

    // Vehicle count should decrease by one
    await expect(
      page.locator('a[href^="/vehicles/"]'),
      "Vehicle list must shrink by one after deletion",
    ).toHaveCount(initialCount - 1, { timeout: 10_000 });
  });

  test("should cancel delete without removing the vehicle", async ({
    page,
  }) => {
    await page.goto("/vehicles");
    const vehiclesPage = new VehiclesPage(page);

    const initialCount = await vehiclesPage.getVehicleCount();

    await vehiclesPage.getDeleteButton(0).click();
    await expect(
      page.getByRole("heading", { name: /delete vehicle/i }),
    ).toBeVisible({ timeout: 5000 });

    await vehiclesPage.cancelDelete();

    await expect(
      page.locator("dialog[open]"),
      "Dialog must close after cancel",
    ).toHaveCount(0, { timeout: 5000 });

    const countAfter = await vehiclesPage.getVehicleCount();
    expect(countAfter, "Vehicle count must be unchanged after cancel").toBe(
      initialCount,
    );
  });
});
