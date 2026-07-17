import { test, expect } from "./fixtures";
import type { ApiHelper, Vehicle } from "../helpers/auth";
import { VehiclesPage } from "../pages/vehiclesPage";

/**
 * Stateful tests for the battery-less "Generic Vehicle" catalog model.
 * Each test resets DB to seed state (3 Maeving vehicles, no generic instance).
 *
 * A generic vehicle (e.g. a petrol bike) has no traction battery: it cannot
 * start EV charge sessions and only supports a 12V maintenance charger.
 */

async function createGenericVehicle(
  api: ApiHelper,
  name?: string,
): Promise<Vehicle> {
  const response = await api.post("/api/vehicles", {
    modelId: "generic",
    ...(name ? { name } : {}),
  });
  if (!response.ok()) {
    throw new Error(
      `Failed to create generic vehicle: ${String(response.status())}`,
    );
  }
  return (await response.json()) as Vehicle;
}

test.describe.serial("Generic Vehicle", () => {
  test("should list Generic Vehicle first in the add-vehicle dialog without battery data", async ({
    page,
  }) => {
    await page.goto("/vehicles");
    const vehiclesPage = new VehiclesPage(page);
    await vehiclesPage.clickAddVehicle();

    const dialog = page.locator("dialog[open]");
    await expect(dialog, "Add vehicle dialog must open").toBeVisible({
      timeout: 5000,
    });

    const modelButtons = dialog
      .getByRole("button")
      .filter({ hasNotText: /cancel/i });
    await expect(
      modelButtons.first(),
      "Generic Vehicle must be the first entry in the model catalog",
    ).toContainText("Generic Vehicle");
    await expect(
      modelButtons.first(),
      "Generic model must show 'No battery' instead of a 0 kWh capacity",
    ).toContainText("No battery");
  });

  test("should create a Generic Vehicle that lists with 'No battery'", async ({
    page,
  }) => {
    await page.goto("/vehicles");
    const vehiclesPage = new VehiclesPage(page);
    const initialCount = await vehiclesPage.getVehicleCount();

    await vehiclesPage.clickAddVehicle();
    const dialog = page.locator("dialog[open]");
    await expect(dialog, "Add vehicle dialog must open").toBeVisible({
      timeout: 5000,
    });
    await dialog
      .getByRole("button")
      .filter({ hasText: /generic vehicle/i })
      .click();

    await expect(
      page.locator("dialog[open]"),
      "Dialog must close after the generic vehicle is created",
    ).toHaveCount(0, { timeout: 10_000 });
    await expect(
      page.locator('a[href^="/vehicles/"]'),
      "Generic vehicle must appear in the list",
    ).toHaveCount(initialCount + 1, { timeout: 10_000 });
    // .first(): the closed add-dialog's model list also contains "No battery"
    // but stays hidden in the DOM.
    await expect(
      page.getByText("No battery").first(),
      "Generic vehicle row must show 'No battery' instead of a capacity",
    ).toBeVisible();
  });

  test("should hide battery specs and stats on the generic vehicle detail page", async ({
    page,
    api,
  }) => {
    const vehicle = await createGenericVehicle(api);
    await page.goto(`/vehicles/${vehicle.id}`);

    await expect(
      page.getByText(/12V maintenance charging only/i),
      "Detail page must explain the vehicle is 12V-maintenance-only",
    ).toBeVisible({ timeout: 10_000 });
    await expect(
      page.getByText("Battery Capacity"),
      "Battery capacity row must be hidden for a battery-less vehicle",
    ).toHaveCount(0);
    await expect(
      page.getByText("CC/CV Charging Profile"),
      "CC/CV charging profile must be hidden for a battery-less vehicle",
    ).toHaveCount(0);
  });

  test("should show the maintenance-only panel instead of the gauge on the dashboard", async ({
    page,
    api,
  }) => {
    await createGenericVehicle(api);
    await page.goto("/dashboard");

    await page
      .getByRole("button", { name: /Generic Vehicle/ })
      .first()
      .click();

    await expect(
      page.getByText(/no 12V maintenance charger configured/i),
      "Maintenance-only panel must prompt to add a 12V charger",
    ).toBeVisible({ timeout: 10_000 });
    await expect(
      page.getByRole("button", { name: /add 12V charger/i }),
      "Add 12V charger button must be offered for a generic vehicle without one",
    ).toBeVisible();
    await expect(
      page.getByTestId("speedometer-gauge-svg"),
      "Charge gauge must not render for a battery-less vehicle",
    ).toHaveCount(0);
    await expect(
      page.getByText("Power Draw"),
      "Charts section must not render for a battery-less vehicle",
    ).toHaveCount(0);
  });

  test("should reject starting an EV charge session for a generic vehicle", async ({
    api,
  }) => {
    // Unique name: this test skips the page fixture (and thus the DB reset),
    // so the default "Generic Vehicle" name from earlier tests may still exist.
    const vehicle = await createGenericVehicle(
      api,
      `Petrol Bike ${String(Date.now())}`,
    );
    const response = await api.post("/api/charge-sessions", {
      vehicleId: vehicle.id,
      startPercent: 20,
      targetPercent: 80,
    });
    expect(
      response.status(),
      "Charge session start must be rejected with 400 for a battery-less vehicle",
    ).toBe(400);
    const body = (await response.json()) as { detail?: string };
    expect(
      body.detail ?? "",
      "Problem detail must explain the vehicle has no battery",
    ).toMatch(/no battery/i);
  });
});
