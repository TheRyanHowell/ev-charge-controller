import type { Page } from "@playwright/test";

import { test, expect } from "./fixtures";

/**
 * Tariff persistence E2E tests - modify per-user tariff state.
 *
 * Configures the electricity tariff (base rate + off-peak window) through the
 * Settings modal and verifies it persists across reloads and is reflected by the
 * backend. Time-weighted cost splitting is covered exhaustively by Go unit tests;
 * here we exercise the configuration UI and persistence end to end.
 */

interface TariffResponse {
  baseRatePence: number;
  offPeakWindows: { start: string; end: string; ratePence: number }[];
}

async function openSettings(page: Page) {
  await page.getByRole("button", { name: "Open settings" }).first().click();
  const dialog = page.locator("dialog[open]");
  await expect(
    dialog.getByText("Electricity tariff"),
    "Tariff section should appear in the Settings modal",
  ).toBeVisible({ timeout: 5_000 });
  return dialog;
}

test.describe.serial("Tariff Persistence", () => {
  test.beforeEach(async ({ page }) => {
    await page.waitForLoadState("load");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
  });

  test("configuring a base rate and off-peak window persists after reload", async ({
    page,
    api,
  }) => {
    const dialog = await openSettings(page);

    // Fill base rate and blur to trigger auto-save.
    await dialog.getByLabel("Base rate (pence per kWh)").fill("30");
    await dialog.getByLabel("Base rate (pence per kWh)").press("Tab");

    // Add an off-peak window (auto-saved immediately by the component).
    await dialog.getByRole("button", { name: "Add off-peak window" }).click();
    // Update the rate of the newly added window (window 2) and blur to trigger auto-save.
    const window2Rate = dialog.getByLabel("Off-peak window 2 rate");
    await window2Rate.fill("8");
    await window2Rate.blur();

    // Poll backend until the new window with rate 8 is persisted.
    await expect
      .poll(
        async () => {
          const r = await api.getJson<TariffResponse>("/api/tariff-settings");
          return r.offPeakWindows.length > 1
            ? r.offPeakWindows[1].ratePence
            : null;
        },
        {
          timeout: 10_000,
          message: "New off-peak window rate should auto-save to 8",
        },
      )
      .toBe(8);

    // Backend persisted the tariff for the current user.
    const saved = await api.getJson<TariffResponse>("/api/tariff-settings");
    expect(saved.baseRatePence, "Base rate should persist").toBe(30);
    expect(
      saved.offPeakWindows,
      "Both off-peak windows should persist",
    ).toEqual([
      { start: "00:30", end: "05:30", ratePence: 7.5 },
      { start: "00:30", end: "04:30", ratePence: 8 },
    ]);

    // Reload and confirm the Settings modal pre-fills the saved tariff.
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page.waitForLoadState("load");

    const reopened = await openSettings(page);
    await expect(
      reopened.getByLabel("Base rate (pence per kWh)"),
      "Base rate should be pre-filled after reload",
    ).toHaveValue("30");
    await expect(
      reopened.getByLabel("Off-peak window 1 rate"),
      "Seeded off-peak window rate should be pre-filled after reload",
    ).toHaveValue("7.5");
    await expect(
      reopened.getByLabel("Off-peak window 2 rate"),
      "New off-peak window rate should be pre-filled after reload",
    ).toHaveValue("8");
  });

  test("removing off-peak windows persists after reload", async ({
    page,
    api,
  }) => {
    const dialog = await openSettings(page);

    // Remove all off-peak windows from end to keep indices stable.
    // removeWindow() auto-saves immediately after removal.
    const removeBtn = dialog.getByRole("button", {
      name: /Remove off-peak window \d+/,
    });
    while (await removeBtn.first().isVisible()) {
      await removeBtn.first().click();
    }

    // Poll backend until all window removals are persisted.
    await expect
      .poll(
        async () => {
          const r = await api.getJson<TariffResponse>("/api/tariff-settings");
          return r.offPeakWindows;
        },
        {
          timeout: 5_000,
          message: "Off-peak windows should be empty after removal",
        },
      )
      .toEqual([]);
  });
});
