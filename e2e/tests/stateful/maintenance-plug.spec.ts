import { test, expect } from "./fixtures";
import type { Plug, Vehicle } from "../helpers/auth";

/**
 * Stateful tests for 12V maintenance plug provisioning and toggle.
 * Each test resets DB to seed state.
 *
 * My RM1 has the seeded 12V Charger (mock-tasmota-3 backed) after reset.
 * My RM1S has only a primary charging plug and no maintenance plug.
 *
 * Tests that require a vehicle with NO pre-existing 12V charger use My RM1S.
 * Tests that exercise the seeded MQTT-backed plug use My RM1.
 */
test.describe.serial("12V Maintenance Plug", () => {
  test("should show '12V Maintenance Charger' section in Settings", async ({
    page,
  }) => {
    // My RM1 has a seeded 12V charger; use My RM1S which has none
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
      "My RM1S must be selected before opening Settings",
    ).toHaveAttribute("aria-pressed", "true");

    await page.getByRole("button", { name: /open settings/i }).click();
    await expect(
      page.getByText(/^12V Maintenance Charger$/i),
      "12V Maintenance Charger section must appear in Settings",
    ).toBeVisible({ timeout: 5000 });
    await expect(
      page.getByText(/no 12V charger/i),
      "Should show 'No 12V charger' when none is provisioned for My RM1S",
    ).toBeVisible();
    await page.keyboard.press("Escape");
  });

  test("should open add 12V charger flow from Settings", async ({ page }) => {
    // My RM1 has a seeded 12V charger; use My RM1S which has none
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
      "My RM1S must be selected before opening Settings",
    ).toHaveAttribute("aria-pressed", "true");

    await page.getByRole("button", { name: /open settings/i }).click();
    await expect(
      page.getByRole("button", { name: /add 12V charger/i }),
    ).toBeVisible({ timeout: 5000 });

    await page.getByRole("button", { name: /add 12V charger/i }).click();

    // Settings closes, AddPlugModal opens in 12V mode
    await expect(page.locator("dialog[open]")).toBeVisible({ timeout: 5000 });
    await expect(
      page.getByRole("heading", { name: /add 12V maintenance charger/i }),
    ).toBeVisible();

    // Should offer Auto-configure, Manual, Cancel (direct 12V mode has no wizard skip step)
    await expect(
      page.getByRole("button", { name: /auto-configure/i }),
      "Auto-configure button must be present in 12V add flow",
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /manual/i }),
      "Manual button must be present in 12V add flow",
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /cancel/i }),
      "Cancel button must replace Skip in direct 12V add flow (not wizard context)",
    ).toBeVisible();

    // Close
    await page.keyboard.press("Escape");
    await expect(page.locator("dialog[open]")).toHaveCount(0);
  });

  test("should provision a 12V maintenance plug via manual flow and show it in Settings", async ({
    page,
    api,
  }) => {
    // Use My RM1S (vehicles[1]) - it has no pre-existing maintenance plug
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[1].id; // My RM1S

    // Provision a maintenance plug directly via API
    const response = await api.post("/api/plugs", {
      name: "12V Charger",
      vehicleId,
      type: "maintenance",
    });
    const plugResult = (await response.json()) as { plug: Plug };
    expect(plugResult.plug.type, "Created plug must be maintenance type").toBe(
      "maintenance",
    );

    // Reload and switch to My RM1S to see the newly provisioned plug
    await page.reload({ waitUntil: "load" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
      "My RM1S must be selected to see its Settings",
    ).toHaveAttribute("aria-pressed", "true");

    // Open Settings - should now show the 12V charger section with the plug name
    await page.getByRole("button", { name: /open settings/i }).click();
    await expect(page.getByText(/^12V Maintenance Charger$/i)).toBeVisible({
      timeout: 5000,
    });
    await expect(
      page.getByText("12V Charger"),
      "Provisioned plug name must appear in 12V section",
    ).toBeVisible();

    // Should show edit-name / delete options within the 12V section
    const maintenance12vSection = page
      .locator("section")
      .filter({ hasText: /12V Maintenance Charger/i });
    await expect(
      maintenance12vSection.getByRole("button", { name: /edit name/i }),
      "Edit name button must appear in the 12V section",
    ).toBeVisible();
    await expect(
      maintenance12vSection.getByRole("button", { name: /delete/i }),
      "Delete button must appear in the 12V section",
    ).toBeVisible();

    await page.keyboard.press("Escape");
  });

  test("should delete a 12V maintenance plug from Settings", async ({
    page,
    api,
  }) => {
    // Use My RM1S (vehicles[1]) - it has no pre-existing maintenance plug
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[1].id; // My RM1S
    await api.post("/api/plugs", {
      name: "12V To Delete",
      vehicleId,
      type: "maintenance",
    });

    // Reload and switch to My RM1S
    await page.reload({ waitUntil: "load" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
      "My RM1S must be selected to see its Settings",
    ).toHaveAttribute("aria-pressed", "true");

    await page.getByRole("button", { name: /open settings/i }).click();
    await expect(page.getByText("12V To Delete")).toBeVisible({
      timeout: 5000,
    });

    // Click Delete in the 12V section specifically
    await page
      .locator("section")
      .filter({ hasText: /12V Maintenance Charger/i })
      .getByRole("button", { name: /delete/i })
      .click();

    // Confirm deletion - the confirmation "Delete" button has visible text content
    // unlike the icon Delete buttons which rely solely on aria-label with no text.
    await expect(page.getByText(/remove this 12V charger/i)).toBeVisible({
      timeout: 3000,
    });
    await page
      .locator("button")
      .filter({ hasText: /^delete$/i })
      .click();

    // Should revert to "No 12V charger"
    await expect(page.getByText(/no 12V charger/i)).toBeVisible({
      timeout: 5000,
    });
    await page.keyboard.press("Escape");
  });

  test("should show reconfigure form for a 12V maintenance plug", async ({
    page,
    api,
  }) => {
    // Use My RM1S (vehicles[1]) - it has no pre-existing maintenance plug
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[1].id; // My RM1S
    await api.post("/api/plugs", {
      name: "12V Reconfigure Test",
      vehicleId,
      type: "maintenance",
    });

    // Reload and switch to My RM1S
    await page.reload({ waitUntil: "load" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
      "My RM1S must be selected to see its Settings",
    ).toHaveAttribute("aria-pressed", "true");

    await page.getByRole("button", { name: /open settings/i }).click();
    await expect(page.getByText("12V Reconfigure Test")).toBeVisible({
      timeout: 5000,
    });

    // My RM1S now has two Configure buttons: one for Driveway Plug, one for 12V.
    // Target the 12V section specifically.
    const section12v = page
      .locator("section")
      .filter({ hasText: /12V Maintenance Charger/i });
    await section12v.getByRole("button", { name: /configure/i }).click();

    await expect(
      page.getByRole("button", { name: /auto-configure/i }),
      "Auto-configure option must appear in configure chooser",
    ).toBeVisible({ timeout: 3000 });
    await expect(
      page.getByRole("button", { name: /^manual/i }),
      "Manual option must appear in configure chooser",
    ).toBeVisible();

    // Select Auto-configure - the AutoConfigureForm should appear
    await page.getByRole("button", { name: /auto-configure/i }).click();
    await expect(
      page.getByPlaceholder(/192\.168\.1\.50/i),
      "IP address field must be visible in the auto-configure form",
    ).toBeVisible({ timeout: 3000 });

    // Cancel returns to the path-select chooser
    await page.getByRole("button", { name: /cancel/i }).click();
    await expect(
      page.getByRole("button", { name: /auto-configure/i }),
      "Chooser must reappear after cancelling auto-configure form",
    ).toBeVisible({ timeout: 3000 });

    // Cancel again closes the chooser
    await page.getByRole("button", { name: /cancel/i }).click();
    await expect(
      section12v.getByRole("button", { name: /configure/i }),
      "Gear icon must reappear after closing chooser",
    ).toBeVisible({ timeout: 3000 });

    await page.keyboard.press("Escape");
  });

  test("should open configure modal for primary charging plug", async ({
    page,
  }) => {
    // My RM1 has a primary charging plug (Garage Plug) and a seeded 12V charger.
    // The primary charger's Configure button is first in the DOM.
    await page.getByRole("button", { name: /open settings/i }).click();
    await expect(
      page.getByText(/^Primary Charger$/i),
      "Primary Charger section must be visible in Settings",
    ).toBeVisible({ timeout: 5000 });

    await page
      .getByRole("button", { name: /configure/i })
      .first()
      .click();

    // Configure modal opens as a separate dialog above the settings modal
    await expect(
      page.getByRole("heading", { name: /^configure$/i }),
      "Configure dialog title must appear",
    ).toBeVisible({ timeout: 3000 });
    await expect(
      page.getByRole("button", { name: /auto-configure/i }),
      "Auto-configure option must be in the configure chooser",
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /^manual/i }),
      "Manual option must be in the configure chooser",
    ).toBeVisible();

    // Navigate into Auto-configure → IP form appears
    await page.getByRole("button", { name: /auto-configure/i }).click();
    await expect(
      page.getByPlaceholder(/192\.168\.1\.50/i),
      "IP address field must appear after selecting Auto-configure",
    ).toBeVisible({ timeout: 3000 });

    // Cancel returns to the chooser (not closes the modal)
    await page.getByRole("button", { name: /cancel/i }).click();
    await expect(
      page.getByRole("button", { name: /auto-configure/i }),
      "Chooser must reappear after cancelling Auto-configure form",
    ).toBeVisible({ timeout: 3000 });

    // Cancel again closes the configure modal; settings modal stays open
    await page.getByRole("button", { name: /cancel/i }).click();
    await expect(
      page.getByText(/^Primary Charger$/i),
      "Settings modal must still be open after closing configure modal",
    ).toBeVisible({ timeout: 3000 });

    await page.keyboard.press("Escape");
  });

  test("should show notification toggles for the selected vehicle", async ({
    page,
  }) => {
    await page.getByRole("button", { name: /open settings/i }).click();

    // Three notification toggles must be visible
    await expect(page.getByText(/charge complete/i)).toBeVisible({
      timeout: 5000,
    });
    await expect(page.getByText(/^charger offline$/i)).toBeVisible();
    await expect(
      page.getByText(/12V maintenance charger offline/i),
    ).toBeVisible();

    // All notification switches should default to on. Scope by the row's
    // own label text (not position) - the General panel has its own Dark
    // mode / Push Notifications toggles above these, so a position-based
    // locator is fragile.
    const dialog = page.locator("dialog");
    const chargeStartedSwitch = dialog
      .getByText(/^charge started$/i)
      .locator("..")
      .locator('[role="switch"]');
    await expect(
      chargeStartedSwitch,
      "Charge started switch must be checked by default",
    ).toHaveAttribute("aria-checked", "true");

    await page.keyboard.press("Escape");
  });
});

/**
 * Tests for the seeded 12V maintenance plug on My RM1 that is backed by
 * mock-tasmota-3. These tests exercise MQTT-driven power toggle, online
 * status display, and per-vehicle gauge circle visibility.
 *
 * The seed provisions seedPlugID3 ("12V Charger", type=maintenance) on My RM1
 * with mock-tasmota-3 (port 8083) providing MQTT connectivity. After each reset
 * the plug is online and its power state reflects the initializer's Power ON.
 */
test.describe.serial("Seeded 12V Maintenance Plug (MQTT-backed)", () => {
  test("should show 12V gauge circle for My RM1 and report it as online", async ({
    page,
  }) => {
    // The maintenance-circle appears only when the selected vehicle has a
    // maintenance plug. My RM1 has the seeded 12V Charger.
    const circle = page.getByTestId("maintenance-circle");
    await expect(
      circle,
      "12V maintenance circle must be visible on the gauge for My RM1",
    ).toBeVisible({ timeout: 20_000 });

    // The plug is MQTT-backed (mock-tasmota-3), so it must be online after reset
    await expect(
      circle,
      "Seeded 12V charger must not be offline after seed reset",
    ).not.toHaveAttribute("aria-label", "12V charger offline");
  });

  test("should toggle 12V power on then off via gauge circle", async ({
    page,
  }) => {
    const circle = page.getByTestId("maintenance-circle");

    // Wait for a stable on/off state before interacting
    await expect(
      circle,
      "12V circle must reach a stable on/off state (not offline) before toggle",
    ).toHaveAttribute("aria-label", /12V charger (on|off) -/i, {
      timeout: 20_000,
    });

    const initialCheckedRaw = await circle.getAttribute("aria-checked");
    expect(
      initialCheckedRaw,
      "12V circle must have aria-checked attribute",
    ).not.toBe(null);
    if (initialCheckedRaw === null) throw new Error("aria-checked is null");
    const initialChecked = initialCheckedRaw;

    // First toggle
    const [firstResp] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes("/api/plugs/") &&
          r.url().includes("/power") &&
          r.request().method() === "PATCH",
        { timeout: 15_000 },
      ),
      circle.click(),
    ]);
    expect(firstResp.status(), "First toggle PATCH must return 200").toBe(200);

    const flippedChecked = initialChecked === "true" ? "false" : "true";
    await expect(
      circle,
      "aria-checked must flip to opposite value after first toggle",
    ).toHaveAttribute("aria-checked", flippedChecked, { timeout: 10_000 });

    // Second toggle - returns to initial state
    const [secondResp] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes("/api/plugs/") &&
          r.url().includes("/power") &&
          r.request().method() === "PATCH",
        { timeout: 15_000 },
      ),
      circle.click(),
    ]);
    expect(secondResp.status(), "Second toggle PATCH must return 200").toBe(
      200,
    );

    await expect(
      circle,
      "aria-checked must return to initial value after second toggle",
    ).toHaveAttribute("aria-checked", initialChecked, { timeout: 10_000 });
  });

  test("should not show 12V gauge circle for My RM1S (no maintenance plug)", async ({
    page,
  }) => {
    // Switch to My RM1S - it has only a primary charging plug, no maintenance plug
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
      "My RM1S must be selected",
    ).toHaveAttribute("aria-pressed", "true");

    // Gauge must still render
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });

    // The 12V circle must be absent - My RM1S has no maintenance plug
    await expect(
      page.getByTestId("maintenance-circle"),
      "12V circle must not appear for My RM1S which has no maintenance plug",
    ).not.toBeVisible();
  });

  test("should show seeded 12V Charger in Settings for My RM1 with online status", async ({
    page,
  }) => {
    await page.getByRole("button", { name: /open settings/i }).click();

    // Seeded plug name confirms the 12V section is showing the plug (not "No 12V charger")
    await expect(
      page.getByText("My RM1 12V"),
      "Seeded plug name must appear in the 12V section",
    ).toBeVisible({ timeout: 5000 });
    await expect(
      page.getByText(/no 12V charger/i),
      "'No 12V charger' must not appear when the seeded plug is present",
    ).not.toBeVisible();

    // Status dot inside the 12V section must show On or Off (never Offline -
    // the plug is MQTT-backed and comes online during seed reset)
    const section12v = page
      .locator("section")
      .filter({ hasText: /12V Maintenance Charger/i });
    const offlineDot = section12v.locator('[aria-label="Offline"]');
    await expect(
      offlineDot,
      "Status dot must not show Offline for the MQTT-backed seeded plug",
    ).not.toBeVisible();

    await page.keyboard.press("Escape");
  });

  test("should reflect 12V toggle in Settings status dot", async ({ page }) => {
    const circle = page.getByTestId("maintenance-circle");

    // Wait for a stable state
    await expect(circle).toHaveAttribute(
      "aria-label",
      /12V charger (on|off) -/i,
      { timeout: 20_000 },
    );
    const initialChecked = await circle.getAttribute("aria-checked");

    // Toggle to the opposite state
    const [resp] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes("/api/plugs/") &&
          r.url().includes("/power") &&
          r.request().method() === "PATCH",
        { timeout: 15_000 },
      ),
      circle.click(),
    ]);
    expect(resp.status(), "Toggle PATCH must return 200").toBe(200);

    const nowOn = initialChecked !== "true";

    await expect(circle).toHaveAttribute(
      "aria-checked",
      nowOn ? "true" : "false",
      { timeout: 10_000 },
    );

    // Open Settings and verify the status dot matches the toggled state
    await page.getByRole("button", { name: /open settings/i }).click();
    await expect(page.getByText(/^12V Maintenance Charger$/i)).toBeVisible({
      timeout: 5000,
    });

    const section12v = page
      .locator("section")
      .filter({ hasText: /12V Maintenance Charger/i });
    const expectedDotLabel = nowOn ? "On" : "Off";
    await expect(
      section12v.locator(`[aria-label="${expectedDotLabel}"]`),
      `Settings status dot must show "${expectedDotLabel}" after toggle`,
    ).toBeVisible({ timeout: 5000 });

    await page.keyboard.press("Escape");
  });
});
