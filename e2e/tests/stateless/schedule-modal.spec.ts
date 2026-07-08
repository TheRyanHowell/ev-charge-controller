import { test, expect } from "./fixtures";

/**
 * Schedule modal E2E tests - read-only UI interactions.
 *
 * These tests only open and close the modal; they do not persist any
 * schedule changes to the database, so they are safe to run in parallel
 * with stateful tests.
 */
test.describe("Schedule Modal UI", () => {
  test.beforeEach(async ({ page }) => {
    // Fixture already navigated. Wait for page and React hydration to finish.
    await expect(
      page.getByTestId("speedometer-gauge-svg"),
      "Dashboard gauge must be visible before interacting",
    ).toBeVisible({ timeout: 15_000 });
    await page.waitForLoadState("load");
  });

  test("schedule circle is visible in the gauge area", async ({ page }) => {
    await expect(
      page.getByTestId("schedule-circle"),
      "Schedule circle should always be rendered in the gauge overlay",
    ).toBeVisible();
  });

  test("schedule circle shows active label when schedule is enabled", async ({
    page,
  }) => {
    // Seed creates an enabled daily schedule at 06:00 for the first plug.
    // With My RM1 selected (assigned to Garage Plug = first plug), the schedule is active.
    await expect(
      page.getByTestId("schedule-circle"),
      "Schedule circle aria-label should indicate the active schedule",
    ).toHaveAttribute("aria-label", /schedule active/i);
  });

  test("clicking schedule circle opens the Charge Schedule modal", async ({
    page,
  }) => {
    await page.getByTestId("schedule-circle").click();

    const dialog = page.locator("dialog[open]");
    await expect(
      dialog,
      "A native <dialog open> element should appear after clicking the schedule circle",
    ).toBeVisible({ timeout: 5_000 });

    await expect(
      dialog.getByRole("heading", { name: "Charge Schedule" }),
      "Modal heading should read 'Charge Schedule'",
    ).toBeVisible();
  });

  test("modal shows Daily and Carbon-aware type tabs", async ({ page }) => {
    await page.getByTestId("schedule-circle").click();
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toBeVisible({ timeout: 5_000 });

    await expect(
      dialog.getByRole("button", { name: "Daily" }),
      "Daily tab must be present",
    ).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Carbon-aware" }),
      "Carbon-aware tab must be present",
    ).toBeVisible();
  });

  test("Daily tab shows a start-time input", async ({ page }) => {
    await page.getByTestId("schedule-circle").click();
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toBeVisible({ timeout: 5_000 });

    // Daily is selected by default
    await expect(
      dialog.getByLabel("Start time"),
      "Start time input should be visible in daily mode",
    ).toBeVisible();
  });

  test("switching to Carbon-aware tab shows earliest and ready-by inputs", async ({
    page,
  }) => {
    await page.getByTestId("schedule-circle").click();
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toBeVisible({ timeout: 5_000 });

    await dialog.getByRole("button", { name: "Carbon-aware" }).click();

    await expect(
      dialog.getByLabel("Earliest"),
      "Earliest start input should appear after switching to Carbon-aware",
    ).toBeVisible();
    await expect(
      dialog.getByLabel("Ready by"),
      "Ready by input should appear after switching to Carbon-aware",
    ).toBeVisible();
  });

  test("Daily tab shows a Two-stage charging toggle, off by default", async ({
    page,
  }) => {
    await page.getByTestId("schedule-circle").click();
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toBeVisible({ timeout: 5_000 });

    // Daily is selected by default; seed schedule has no readyBy set.
    await expect(
      dialog.getByRole("switch", { name: "Two-stage charging" }),
      "Two-stage charging toggle should be present on the Daily tab",
    ).toBeVisible();
    await expect(
      dialog.getByRole("switch", { name: "Two-stage charging" }),
      "Two-stage charging should be off when the seed schedule has no readyBy",
    ).toHaveAttribute("aria-checked", "false");
  });

  test("toggling Two-stage charging reveals a Ready by input on the Daily tab", async ({
    page,
  }) => {
    await page.getByTestId("schedule-circle").click();
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toBeVisible({ timeout: 5_000 });

    await expect(dialog.getByLabel("Ready by")).not.toBeVisible();

    await dialog.getByRole("switch", { name: "Two-stage charging" }).click();

    await expect(
      dialog.getByLabel("Ready by"),
      "Ready by input should appear once two-stage charging is enabled",
    ).toBeVisible();
  });

  test("Skip button closes the modal", async ({ page }) => {
    await page.getByTestId("schedule-circle").click();
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toBeVisible({ timeout: 5_000 });

    await dialog.getByRole("button", { name: /skip/i }).click();

    await expect(
      page.locator("dialog[open]"),
      "Modal should close after clicking Skip",
    ).toHaveCount(0, { timeout: 5_000 });
  });

  test("X button closes the modal", async ({ page }) => {
    await page.getByTestId("schedule-circle").click();
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toBeVisible({ timeout: 5_000 });

    await dialog
      .getByRole("button", { name: "Close schedule settings" })
      .click();

    await expect(
      page.locator("dialog[open]"),
      "Modal should close after clicking the X button",
    ).toHaveCount(0, { timeout: 5_000 });
  });

  test("Escape key closes the modal", async ({ page }) => {
    await page.getByTestId("schedule-circle").click();
    const dialog = page.locator("dialog[open]");
    await expect(dialog).toBeVisible({ timeout: 5_000 });

    await page.keyboard.press("Escape");

    await expect(
      page.locator("dialog[open]"),
      "Modal should close on Escape key press",
    ).toHaveCount(0, { timeout: 5_000 });
  });
});
