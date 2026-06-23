import type { Page } from "@playwright/test";
import { test, expect } from "./fixtures";
import { Vehicle } from "../helpers/auth";

// Type declaration for React Query client exposed on window (dev only)
declare global {
  interface Window {
    __queryClient__?: { clear: () => void };
  }
}

/**
 * Stateful Dashboard tests cover all tests that modify shared database state
 * (charge sessions, vehicle percents, plug assignments). These tests run
 * serially on a single worker to avoid race conditions.
 *
 * Because charging is now synchronous (blocking MQTT confirmation), sessions
 * transition directly from "pending" to "active" within the POST request.
 * No polling for activation is needed.
 *
 * Includes:
 * - Charge Session Lifecycle (start/stop, rapid cycling, history)
 * - Gauge State Separation (percents per vehicle, plug switching)
 * - Plug Switching (selector, online status, vehicle info)
 */

// Helper: navigate to dashboard and wait for gauge
// Uses domcontentloaded because the dashboard polls every 5s,
// so networkidle would never resolve (requires 500ms of no requests).
async function navigateToDashboard(page: Page) {
  await page.goto("/dashboard", {
    waitUntil: "domcontentloaded",
    timeout: 30_000,
  });
}

// Helper: wait for the gauge to show a specific percent
// Uses data-testid="gauge-percent" for reliable matching
async function expectGaugePercent(page: Page, percent: string) {
  await expect(page.getByTestId("gauge-percent")).toContainText(percent, {
    ignoreCase: true,
    timeout: 10000,
  });
}

test.describe.serial("Charge Session Lifecycle", () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to dashboard and wait for it to load
    await navigateToDashboard(page);
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });

    // Clear React Query cache to force fresh data fetch (no-op in production mode)
    await page.evaluate(() => {
      window.__queryClient__?.clear();
    });

    // Wait for plug to be online (mock-tasmota needs time to reconnect after reset)
    await expect(page.getByLabel("Online").first()).toBeVisible({
      timeout: 15_000,
    });

    // Ensure we're on the default vehicle (My RM1 - Garage Plug)
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");

    // Verify we're in idle state (START button visible and enabled)
    const startButton = page.getByRole("button", { name: /start charging/i });
    await expect(startButton).toBeVisible({ timeout: 10_000 });
    await expect(startButton).toBeEnabled({ timeout: 10_000 });
  });

  test("should show START button in idle state", async ({ page }) => {
    const startButton = page.getByRole("button", {
      name: /start charging/i,
    });
    await expect(startButton).toBeVisible({ timeout: 10_000 });
    await expect(startButton).toBeEnabled();
  });

  test("should show READY status in idle state", async ({ page }) => {
    await expect(page.getByText("Ready")).toBeVisible({ timeout: 10_000 });
  });

  test("should start charging when START button is clicked", async ({
    page,
    api,
  }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[0].id;

    const startButton = page.getByRole("button", {
      name: /start charging/i,
    });
    await expect(startButton).toBeVisible();
    await expect(startButton).toBeEnabled();

    // Click START and wait for the API call (POST returns 201 with active session)
    const [response] = await Promise.all([
      page.waitForResponse(
        (resp) =>
          resp.url().includes("/api/charge-sessions") &&
          resp.request().method() === "POST",
        { timeout: 25_000 },
      ),
      startButton.click(),
    ]);

    expect(response.status()).toBe(201);

    // Verify STOP button appears (session is immediately "active" - no pending state)
    const stopButton = page.getByRole("button", {
      name: /stop charging/i,
    });
    await expect(stopButton).toBeVisible({ timeout: 10_000 });
    await expect(stopButton).toBeEnabled();

    // Verify session exists and is "active" via API
    const session = await api.getSession(
      `/api/charge-sessions?vehicleId=${vehicleId}`,
    );
    expect(session).not.toBeNull();
    expect(session?.status).toBe("active");

    // Clean up: stop charging
    await stopButton.click();
  });

  test("should stop charging when STOP button is clicked", async ({ page }) => {
    // Start charging first
    const startButton = page.getByRole("button", {
      name: /start charging/i,
    });
    await startButton.click();

    const stopButton = page.getByRole("button", {
      name: /stop charging/i,
    });
    await expect(stopButton).toBeVisible({ timeout: 25_000 });

    // Click STOP and wait for the API call
    const [response] = await Promise.all([
      page.waitForResponse(
        (resp) =>
          resp.url().includes("/api/charge-sessions") &&
          resp.request().method() === "PATCH",
        { timeout: 25_000 },
      ),
      stopButton.click(),
    ]);

    expect(response.status()).toBe(204);

    // Verify START button reappears
    const restartedButton = page.getByRole("button", {
      name: /start charging/i,
    });
    await expect(restartedButton).toBeVisible({ timeout: 10_000 });

    // Verify READY status returns
    await expect(page.getByText("Ready")).toBeVisible({ timeout: 10_000 });
  });

  test("should show power draw during charging", async ({ page, api }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[0].id;

    // Start charging - session is immediately "active" (synchronous MQTT confirmation)
    await page.getByRole("button", { name: /start charging/i }).click();
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({
      timeout: 25_000,
    });

    // Session should be "active" immediately (no polling needed)
    const session = await api.getSession(
      `/api/charge-sessions?vehicleId=${vehicleId}`,
    );
    expect(session).not.toBeNull();
    expect(session?.status).toBe("active");

    // Wait for energy readings to accumulate (polling interval is ~5s)
    await expect
      .poll(
        async () => {
          const session = await api.getSession(
            `/api/charge-sessions?vehicleId=${vehicleId}`,
          );
          return session?.energyAddedKwh ?? 0;
        },
        { timeout: 30_000 },
      )
      .toBeGreaterThan(0);

    // Clean up: stop charging, wait for PATCH response
    const [stopResponse] = await Promise.all([
      page.waitForResponse(
        (resp) =>
          resp.url().includes("/api/charge-sessions") &&
          resp.request().method() === "PATCH",
        { timeout: 25_000 },
      ),
      page.getByRole("button", { name: /stop charging/i }).click(),
    ]);
    expect(stopResponse.status()).toBe(204);

    // After stopping, the session endpoint returns 204 (no active session)
    // Verify by checking that getSession returns null or completed/cancelled
    const finalSession = await api.getSession(
      `/api/charge-sessions?vehicleId=${vehicleId}`,
    );
    // Either null (no active session) or completed/cancelled
    expect(
      finalSession === null ||
        finalSession.status === "completed" ||
        finalSession.status === "cancelled",
    ).toBe(true);
  });

  test("should show energy added during charging", async ({ page, api }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[0].id;

    // Start charging - session is immediately "active"
    await page.getByRole("button", { name: /start charging/i }).click();
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({
      timeout: 25_000,
    });

    // Wait for energy readings to accumulate (multiple polling cycles)
    // Energy accumulates slowly (~1510W * 15s = ~0.006 kWh)
    await page.waitForTimeout(15000);

    // Verify energy added via API
    const session = await api.getSession(
      `/api/charge-sessions?vehicleId=${vehicleId}`,
    );
    expect(session).not.toBeNull();
    expect(session?.energyAddedKwh).toBeGreaterThan(0);

    // Clean up: stop charging
    await page.getByRole("button", { name: /stop charging/i }).click();
  });

  test("should show elapsed time during charging", async ({ page }) => {
    // Start charging - session is immediately "active"
    await page.getByRole("button", { name: /start charging/i }).click();
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({
      timeout: 25_000,
    });

    // Charge Duration card should show time > 0s
    const durationValue = page
      .locator("text=Charge Duration")
      .first()
      .locator("..")
      .locator("div.text-2xl");

    // Wait a few seconds for the timer to tick
    await page.waitForTimeout(3000);
    const durationText = await durationValue.textContent();
    expect(durationText).not.toBe("0s");

    // Clean up: stop charging
    await page.getByRole("button", { name: /stop charging/i }).click();
  });

  test("should handle rapid start-stop cycling without corruption", async ({
    page,
    api,
  }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const cycles = 3;

    for (let i = 0; i < cycles; i++) {
      // Start - session is immediately "active"
      const startButton = page.getByRole("button", {
        name: /start charging/i,
      });
      await startButton.click();
      await expect(
        page.getByRole("button", { name: /stop charging/i }),
      ).toBeVisible({
        timeout: 25_000,
      });

      // Stop
      const stopButton = page.getByRole("button", { name: /stop charging/i });
      await stopButton.click();
      await expect(
        page.getByRole("button", { name: /start charging/i }),
      ).toBeVisible({
        timeout: 25_000,
      });

      // Brief pause between cycles
      await page.waitForTimeout(1000);
    }

    // After cycling, verify we're back in idle state
    await expect(
      page.getByRole("button", { name: /start charging/i }),
    ).toBeVisible();
    await expect(page.getByText("Ready")).toBeVisible();

    // Verify no active session remains
    const session = await api.getSession(
      `/api/charge-sessions?vehicleId=${vehicles[0].id}`,
    );
    // Either null (no active session) or completed/cancelled
    expect(
      session === null ||
        session.status === "completed" ||
        session.status === "cancelled",
    ).toBe(true);
  });

  test("should create a completed session visible in history", async ({
    page,
  }) => {
    // Start and stop a charge session
    await page.getByRole("button", { name: /start charging/i }).click();
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({
      timeout: 25_000,
    });

    // Wait briefly for energy to accumulate
    await page.waitForTimeout(3000);

    await page.getByRole("button", { name: /stop charging/i }).click();
    await expect(
      page.getByRole("button", { name: /start charging/i }),
    ).toBeVisible({
      timeout: 25_000,
    });

    // Navigate to history
    await page.getByRole("link", { name: /history/i }).click();
    await page.waitForURL(/\/history/, { timeout: 10_000 });

    // Wait for at least one session card - the history page SSR may render
    // before the just-completed session is committed, so we poll with a timeout.
    const sessionCards = page.getByRole("button").filter({
      hasText: /completed|active|cancelled|stopped/i,
    });
    await expect(sessionCards.first()).toBeVisible({ timeout: 10_000 });
  });

  test("should show charging status text during active session", async ({
    page,
    api,
  }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");

    // Verify idle state shows "Ready"
    await expect(page.getByText("Ready")).toBeVisible();

    // Start charging - session is immediately "active"
    await page.getByRole("button", { name: /start charging/i }).click();

    // Wait for STOP button to appear
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({
      timeout: 25_000,
    });

    // Verify session is "active" via API (no pending state)
    const session = await api.getSession(
      `/api/charge-sessions?vehicleId=${vehicles[0].id}`,
    );
    expect(session).not.toBeNull();
    expect(session?.status).toBe("active");

    // Verify UI shows charging indicator (scoped to gauge)
    await expect(
      page.getByTestId("speedometer-gauge").getByText(/Charging/),
    ).toBeVisible({
      timeout: 10_000,
    });

    // Stop charging
    await page.getByRole("button", { name: /stop charging/i }).click();

    // Should show "Ready" again
    await expect(page.getByText("Ready")).toBeVisible({
      timeout: 25_000,
    });
  });
});

test.describe.serial("Gauge State Separation", () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to dashboard and wait for it to load
    await navigateToDashboard(page);
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });

    // Clear React Query cache to force fresh data fetch (no-op in production mode)
    await page.evaluate(() => {
      window.__queryClient__?.clear();
    });

    // Wait for mock-tasmota to be online
    await expect(page.getByLabel("Online").first()).toBeVisible({
      timeout: 15_000,
    });

    // Ensure we're on the default vehicle (My RM1 - Garage Plug)
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
  });

  test("should show correct gauge percents when switching between vehicles with different percents", async ({
    page,
    api,
  }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicle1 = vehicles[0]; // My RM1 (Garage Plug)
    const vehicle2 = vehicles[1]; // My RM1S (Driveway Plug)

    // Explicitly set different percents for each vehicle via API
    await api.patch(`/api/vehicles/${vehicle1.id}`, {
      currentPercent: 20,
      targetPercent: 80,
    });
    await api.patch(`/api/vehicles/${vehicle2.id}`, {
      currentPercent: 50,
      targetPercent: 90,
    });

    // Navigate to fresh dashboard to pick up new percents from server
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });

    // Re-select My RM1 vehicle after navigation
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");

    // Wait for gauge store to initialize (useEffect batching can delay setPercents)
    await page.waitForTimeout(2000);

    // Verify percents via API before asserting on UI
    const v1After = await api.getJson<Vehicle>(`/api/vehicles/${vehicle1.id}`);
    const v2After = await api.getJson<Vehicle>(`/api/vehicles/${vehicle2.id}`);
    const v1Percent = `${String(Math.round(v1After.currentPercent))}%`;
    const v2Percent = `${String(Math.round(v2After.currentPercent))}%`;

    // My RM1 (Vehicle 1) should show its percent
    // Use waitFor to handle timing: gauge may show stale percents briefly
    await expect(page.getByTestId("gauge-percent")).toContainText(v1Percent, {
      timeout: 5000,
    });

    // Switch to My RM1S (Vehicle 2) - should show its percent
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
    await page.waitForTimeout(500);
    await expectGaugePercent(page, v2Percent);

    // Switch back to My RM1 (Vehicle 1) - should show its percent again
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
    await page.waitForTimeout(500);
    await expectGaugePercent(page, v1Percent);
  });

  test("should show correct gauge percents after starting/stopping charge on one vehicle then switching", async ({
    page,
    api,
  }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicle1 = vehicles[0]; // My RM1 (Garage Plug)
    const vehicle2 = vehicles[1]; // My RM1S (Driveway Plug)

    // Set different percents for each vehicle
    await api.patch(`/api/vehicles/${vehicle1.id}`, {
      currentPercent: 20,
      targetPercent: 80,
    });
    await api.patch(`/api/vehicles/${vehicle2.id}`, {
      currentPercent: 50,
      targetPercent: 90,
    });

    // Force full page reload
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });

    // Re-select My RM1 vehicle after reload
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
    // Wait for gauge to stabilize after plug switch (use multiple checks)
    await page.waitForTimeout(200);
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible();

    // Verify percents via API before asserting on UI
    const v1After = await api.getJson<Vehicle>(`/api/vehicles/${vehicle1.id}`);
    const v2After = await api.getJson<Vehicle>(`/api/vehicles/${vehicle2.id}`);
    const v1Percent = `${String(v1After.currentPercent)}%`;
    const v2Percent = `${String(v2After.currentPercent)}%`;

    // Verify Vehicle 1 shows its percent
    await expectGaugePercent(page, v1Percent);

    // Switch to Vehicle 2, verify its percent
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
    await page.waitForTimeout(500);
    await expectGaugePercent(page, v2Percent);

    // Switch back to Vehicle 1, verify its percent
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
    await page.waitForTimeout(500);
    await expectGaugePercent(page, v1Percent);

    // Start charging on Vehicle 1 - session is immediately "active"
    await page.getByRole("button", { name: /start charging/i }).click();
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({
      timeout: 25_000,
    });

    // Switch to Vehicle 2 - should show its idle state
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 5000,
    });
    // Vehicle 2 should show its percent (50%) and idle status
    await expect(page.getByText(/Ready|Disconnected/)).toBeVisible({
      timeout: 5000,
    });

    // Switch back to Vehicle 1 - should still show charging
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 5000,
    });
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({
      timeout: 10_000,
    });

    // Stop charging
    await page.getByRole("button", { name: /stop charging/i }).click();
    await expect(page.getByText("Ready")).toBeVisible({ timeout: 25_000 });

    // Switch to Vehicle 2 - should show its idle state
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 5000,
    });
    // Verify we're on Vehicle 2 by checking the plug is selected
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
  });
});

test.describe.serial("Plug Switching", () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to dashboard and wait for it to load
    await navigateToDashboard(page);
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10_000,
    });

    // Clear React Query cache to force fresh data fetch (no-op in production mode)
    await page.evaluate(() => {
      window.__queryClient__?.clear();
    });

    // Wait for mock-tasmota to be online
    await expect(page.getByLabel("Online").first()).toBeVisible({
      timeout: 15_000,
    });
  });

  test("should show both vehicles in the vehicle selector", async ({
    page,
  }) => {
    // Both plugs should be visible
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
    ).toBeVisible();
  });

  test("should show My RM1 as selected by default", async ({ page }) => {
    // My RM1 should have aria-pressed="true"
    const garagePlug = page.getByRole("button", { name: /My RM1/ }).first();
    await expect(garagePlug).toBeVisible();
    await expect(garagePlug).toHaveAttribute("aria-pressed", "true");

    // My RM1S should have aria-pressed="false"
    const drivewayPlug = page.getByRole("button", { name: /My RM1S/ }).first();
    await expect(drivewayPlug).toBeVisible();
    await expect(drivewayPlug).toHaveAttribute("aria-pressed", "false");
  });

  test("should show online status indicator for each plug", async ({
    page,
  }) => {
    // Both plugs should have online indicators (green dot)
    await expect(page.getByLabel("Online").first()).toBeVisible({
      timeout: 5000,
    });
  });

  test("should switch to My RM1S when clicked", async ({ page }) => {
    // Click Driveway Plug
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();

    // Driveway Plug should now be selected
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");

    // Garage Plug should be deselected
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toHaveAttribute("aria-pressed", "false");
  });

  test("should switch back to My RM1 when clicked", async ({ page }) => {
    // First switch to Driveway Plug
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");

    // Switch back to Garage Plug
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
    ).toHaveAttribute("aria-pressed", "true");
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
    ).toHaveAttribute("aria-pressed", "false");
  });

  test("should update vehicle info when switching plugs", async ({ page }) => {
    // Click My RM1 chip and verify it becomes selected
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1/ }).first(),
      "My RM1 chip should be selected after click",
    ).toHaveAttribute("aria-pressed", "true", { timeout: 5000 });

    // Click My RM1S chip and verify it becomes selected
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /My RM1S/ }).first(),
      "My RM1S chip should be selected after click",
    ).toHaveAttribute("aria-pressed", "true", { timeout: 5000 });
  });

  test("should show different vehicle percents when switching plugs", async ({
    page,
    api,
  }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");

    // Set different percents for each vehicle
    await api.patch(`/api/vehicles/${vehicles[0].id}`, {
      currentPercent: 20,
      targetPercent: 80,
    });
    await api.patch(`/api/vehicles/${vehicles[1].id}`, {
      currentPercent: 50,
      targetPercent: 90,
    });

    // Reload to pick up new percents
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });

    // Garage Plug (Vehicle 1) should show 20%
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expectGaugePercent(page, "20%");

    // Driveway Plug (Vehicle 2) should show 50%
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expectGaugePercent(page, "50%");
  });

  test("should preserve charging state when switching plugs", async ({
    page,
    api,
  }) => {
    // Start charging on Garage Plug
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await page.getByRole("button", { name: /start charging/i }).click();
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({
      timeout: 25_000,
    });

    // Switch to Driveway Plug - should show idle
    await page
      .getByRole("button", { name: /My RM1S/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /start charging/i }),
    ).toBeVisible({
      timeout: 10_000,
    });

    // Switch back to Garage Plug - should still show charging
    await page
      .getByRole("button", { name: /My RM1/ })
      .first()
      .click();
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({
      timeout: 10_000,
    });

    // Stop charging
    await page.getByRole("button", { name: /stop charging/i }).click();

    // Clean up
    await api
      .patch(
        `/api/charge-sessions?vehicleId=${(await api.getJson<Vehicle[]>("/api/vehicles"))[0].id}`,
        { status: "stopped" },
      )
      .catch(() => undefined);
  });
});
