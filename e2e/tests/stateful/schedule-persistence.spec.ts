import type { Page } from "@playwright/test";

import { test, expect } from "./fixtures";

/**
 * Schedule persistence E2E tests - modify database state.
 *
 * These tests save schedules and verify they survive page reloads.
 * They run serially on a single worker; each test starts from the
 * seed-reset state provided by the stateful fixture.
 */

async function openScheduleModal(page: Page) {
  await page.getByTestId("schedule-circle").click();
  const dialog = page.locator("dialog[open]");
  await expect(
    dialog,
    "Schedule modal should open after clicking the circle",
  ).toBeVisible({ timeout: 5_000 });
  return dialog;
}

test.describe.serial("Schedule Persistence", () => {
  test.beforeEach(async ({ page }) => {
    // Stateful fixture already reset DB and navigated to /dashboard.
    // Wait for React hydration so click handlers are attached.
    await page.waitForLoadState("load");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
  });

  test("saving a daily schedule updates the circle and persists after reload", async ({
    page,
  }) => {
    const dialog = await openScheduleModal(page);

    // Seed state has enabled=true, type=daily - just update the start time
    await dialog.getByLabel("Start time").fill("03:00");

    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(
      page.locator("dialog[open]"),
      "Modal should close after saving",
    ).toHaveCount(0, { timeout: 5_000 });

    // Circle should immediately reflect the active schedule
    await expect(
      page.getByTestId("schedule-circle"),
      "Circle should show active daily schedule with the configured start time",
    ).toHaveAttribute("aria-label", /Schedule active.*starts at.*03:00/, {
      timeout: 5_000,
    });

    // Reload and verify the schedule survived
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });

    await expect(
      page.getByTestId("schedule-circle"),
      "Daily schedule should persist after reload",
    ).toHaveAttribute("aria-label", /Schedule active.*starts at.*03:00/);
  });

  test("saving a carbon-aware schedule updates the circle and persists after reload", async ({
    page,
  }) => {
    const dialog = await openScheduleModal(page);

    // Seed state has enabled=true, type=daily - switch type and set window times
    await dialog.getByRole("button", { name: "Carbon-aware" }).click();
    await expect(dialog.getByLabel("Earliest")).toBeVisible();
    await dialog.getByLabel("Earliest").fill("22:00");
    await dialog.getByLabel("Ready by").fill("06:00");

    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(
      page.locator("dialog[open]"),
      "Modal should close after saving",
    ).toHaveCount(0, { timeout: 5_000 });

    // Circle should show the ready-by time for carbon-aware schedules
    await expect(
      page.getByTestId("schedule-circle"),
      "Circle should show active carbon-aware schedule with the ready-by time",
    ).toHaveAttribute("aria-label", /Schedule active.*ready by.*06:00/, {
      timeout: 5_000,
    });

    // Reload and verify the schedule survived
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });

    await expect(
      page.getByTestId("schedule-circle"),
      "Carbon-aware schedule should persist after reload",
    ).toHaveAttribute("aria-label", /Schedule active.*ready by.*06:00/);
  });

  test("opening modal pre-fills saved values for an existing schedule", async ({
    page,
    api,
  }) => {
    // Seed a known schedule via API so the UI can be verified in isolation.
    const plugs = await api.getJson<{ id: string }[]>("/api/plugs");
    const plugId = plugs[0]?.id;
    if (!plugId) throw new Error("No plug found in seed data");

    await api.patch(`/api/plugs/${plugId}/schedule`, {
      type: "daily",
      time: "05:30",
      enabled: true,
    });

    // Reload so the server component re-fetches the schedule.
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page.waitForLoadState("load");

    const dialog = await openScheduleModal(page);

    // Modal should pre-fill from the saved schedule.
    await expect(
      dialog.getByRole("switch"),
      "Enable toggle should reflect saved enabled state",
    ).toHaveAttribute("aria-checked", "true");
    await expect(
      dialog.getByLabel("Start time"),
      "Start time input should reflect saved schedule time",
    ).toHaveValue("05:30");
  });

  test("disabling a schedule removes the active label from the circle", async ({
    page,
    api,
  }) => {
    // First create an enabled schedule via API.
    const plugs = await api.getJson<{ id: string }[]>("/api/plugs");
    const plugId = plugs[0]?.id;
    if (!plugId) throw new Error("No plug found in seed data");

    await api.patch(`/api/plugs/${plugId}/schedule`, {
      type: "daily",
      time: "03:00",
      enabled: true,
    });

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page.waitForLoadState("load");

    // Open modal, toggle schedule off, save.
    const dialog = await openScheduleModal(page);
    await dialog.getByRole("switch").click(); // toggle off (was on)
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(page.locator("dialog[open]")).toHaveCount(0, {
      timeout: 5_000,
    });

    // Circle should show the disabled state - schedule still has its time,
    // it's just no longer active.
    await expect(
      page.getByTestId("schedule-circle"),
      "Circle should show disabled state with configured time after disabling",
    ).toHaveAttribute(
      "aria-label",
      "Schedule configured but disabled - 03:00",
      {
        timeout: 5_000,
      },
    );
  });
});
