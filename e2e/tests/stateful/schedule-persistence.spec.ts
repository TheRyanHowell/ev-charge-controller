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

  test("saving a daily schedule with two-stage charging persists readyBy after reload", async ({
    page,
  }) => {
    const dialog = await openScheduleModal(page);

    await dialog.getByLabel("Start time").fill("01:00");
    await dialog.getByRole("switch", { name: "Two-stage charging" }).click();
    await dialog.getByLabel("Ready by").fill("07:00");

    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(
      page.locator("dialog[open]"),
      "Modal should close after saving",
    ).toHaveCount(0, { timeout: 5_000 });

    // The circle label is driven by schedule.time regardless of readyBy.
    await expect(
      page.getByTestId("schedule-circle"),
      "Circle should show active daily schedule with the configured start time",
    ).toHaveAttribute("aria-label", /Schedule active.*starts at.*01:00/, {
      timeout: 5_000,
    });

    // Reload and verify readyBy survived by re-opening the modal.
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page.waitForLoadState("load");

    const reopened = await openScheduleModal(page);
    await expect(
      reopened.getByRole("switch", { name: "Two-stage charging" }),
      "Two-stage charging should still be enabled after reload",
    ).toHaveAttribute("aria-checked", "true");
    await expect(
      reopened.getByLabel("Ready by"),
      "Ready by time should persist after reload",
    ).toHaveValue("07:00");
  });

  test("saving a daily schedule with two-stage charging shows the estimated plan", async ({
    page,
    api,
  }) => {
    // Unlike carbon-aware, daily two-stage has no forecast dependency - the
    // estimate is pure arithmetic against the vehicle's spec, so it's safe
    // to assert real rendered values here. Rather than hand-predicting the
    // vehicle-spec-dependent minutes (fragile if seed data ever changes),
    // read the backend's own computed plan from the PATCH response and
    // assert the UI renders exactly that. stage1Start and stage2End are
    // additionally pinned to exact values since those never depend on the
    // vehicle's charge-duration estimate (formatTime strips the calendar
    // day, so "next occurrence of 01:00" is always literally "01:00").
    const plugs = await api.getJson<{ id: string }[]>("/api/plugs");
    const plugId = plugs[0]?.id;
    if (!plugId) throw new Error("No plug found in seed data");

    const saveRes = await api.patch(`/api/plugs/${plugId}/schedule`, {
      type: "daily",
      time: "01:00",
      readyBy: "07:00",
      enabled: true,
    });
    const saved = (await saveRes.json()) as {
      estimatedPlan?: {
        stage1Start: string;
        stage1End: string;
        stage2Start: string;
        stage2End: string;
      };
    };
    const plan = saved.estimatedPlan;
    if (!plan) throw new Error("Expected estimatedPlan in the PATCH response");
    expect(plan.stage1Start).toBe("01:00");
    expect(plan.stage2End).toBe("07:00");
    const timeRe = /^([01]\d|2[0-3]):[0-5]\d$/;
    expect(plan.stage1End).toMatch(timeRe);
    expect(plan.stage2Start).toMatch(timeRe);

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page.waitForLoadState("load");

    const dialog = await openScheduleModal(page);
    const planPanel = dialog.getByTestId("estimated-plan");
    await expect(
      planPanel,
      "Estimated plan panel should render for a daily two-stage schedule matching the backend's computed plan",
    ).toBeVisible({ timeout: 5_000 });
    await expect(planPanel).toContainText(`Stage 1: ${plan.stage1Start}`);
    await expect(planPanel).toContainText(plan.stage1End);
    await expect(planPanel).toContainText(`Hold until ${plan.stage2Start}`);
    await expect(planPanel).toContainText(`Stage 2: ${plan.stage2Start}`);
    await expect(planPanel).toContainText(plan.stage2End);
  });

  test("daily readyBy equal to start time shows a validation error", async ({
    page,
  }) => {
    const dialog = await openScheduleModal(page);

    await dialog.getByLabel("Start time").fill("04:00");
    await dialog.getByRole("switch", { name: "Two-stage charging" }).click();
    await dialog.getByLabel("Ready by").fill("04:00");

    await dialog.getByRole("button", { name: "Save" }).click();

    await expect(
      dialog.getByRole("alert"),
      "Form should show a validation error instead of saving",
    ).toHaveText("Ready by must differ from start time.");
    await expect(
      dialog,
      "Modal should stay open when validation fails",
    ).toBeVisible();
  });

  test("opening modal pre-fills readyBy for an existing two-stage schedule", async ({
    page,
    api,
  }) => {
    // Seed a two-stage daily schedule via API so the UI can be verified in isolation.
    const plugs = await api.getJson<{ id: string }[]>("/api/plugs");
    const plugId = plugs[0]?.id;
    if (!plugId) throw new Error("No plug found in seed data");

    await api.patch(`/api/plugs/${plugId}/schedule`, {
      type: "daily",
      time: "02:00",
      readyBy: "08:00",
      enabled: true,
    });

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page.waitForLoadState("load");

    const dialog = await openScheduleModal(page);

    await expect(
      dialog.getByRole("switch", { name: "Two-stage charging" }),
      "Two-stage charging toggle should reflect the saved readyBy",
    ).toHaveAttribute("aria-checked", "true");
    await expect(
      dialog.getByLabel("Ready by"),
      "Ready by input should reflect the saved schedule",
    ).toHaveValue("08:00");
  });

  test("saving a carbon-aware schedule with two-stage charging persists twoStage after reload", async ({
    page,
  }) => {
    const dialog = await openScheduleModal(page);

    await dialog.getByRole("button", { name: "Carbon-aware" }).click();
    await dialog.getByLabel("Earliest").fill("22:00");
    await dialog.getByLabel("Ready by").fill("06:00");
    await dialog
      .getByRole("switch", { name: "Carbon-aware two-stage charging" })
      .click();

    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(
      page.locator("dialog[open]"),
      "Modal should close after saving",
    ).toHaveCount(0, { timeout: 5_000 });

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page.waitForLoadState("load");

    // Reopening pre-fills from the saved schedule.type=carbon_aware, so the
    // Carbon-aware tab is already selected without needing to click it.
    const reopened = await openScheduleModal(page);
    await expect(
      reopened.getByRole("switch", { name: "Carbon-aware two-stage charging" }),
      "Carbon-aware two-stage toggle should still be enabled after reload",
    ).toHaveAttribute("aria-checked", "true");
  });

  test("opening modal pre-fills twoStage for an existing carbon-aware two-stage schedule", async ({
    page,
    api,
  }) => {
    // Seed a two-stage carbon-aware schedule via API so the UI can be verified in isolation.
    const plugs = await api.getJson<{ id: string }[]>("/api/plugs");
    const plugId = plugs[0]?.id;
    if (!plugId) throw new Error("No plug found in seed data");

    await api.patch(`/api/plugs/${plugId}/schedule`, {
      type: "carbon_aware",
      windowStart: "22:00",
      windowEnd: "06:00",
      twoStage: true,
      enabled: true,
    });

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await page.waitForLoadState("load");

    // schedule.type is already carbon_aware, so that tab is pre-selected.
    const dialog = await openScheduleModal(page);

    await expect(
      dialog.getByRole("switch", { name: "Carbon-aware two-stage charging" }),
      "Carbon-aware two-stage toggle should reflect the saved twoStage flag",
    ).toHaveAttribute("aria-checked", "true");
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
      dialog.getByRole("switch", { name: "Enabled" }),
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
    await dialog.getByRole("switch", { name: "Enabled" }).click(); // toggle off (was on)
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
