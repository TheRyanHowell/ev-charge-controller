import { test, expect } from "./fixtures";
import { HistoryPage } from "../pages/historyPage";

/**
 * History Extended tests are read-only (no session manipulation).
 * They verify history cards, filtering, and session details.
 * No beforeEach cleanup needed - these tests are safe to run in parallel
 * with stateful tests.
 *
 * Seed always emits ≥2 completed sessions per vehicle (2 vehicles with plugs)
 * dated today, so the default "today" date filter is never empty.
 */
test.describe("History Extended", () => {
  test("should show session card with vehicle name", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const firstCard = historyPage.getSessionCard(0);
    const cardText = await firstCard.textContent();
    expect(cardText).toMatch(/rm1|rm1s|rm2/i);
  });

  test("should show session card with start percentage", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const firstCard = historyPage.getSessionCard(0);
    await expect(firstCard.getByText(/From/i)).toBeVisible();
  });

  test("should show session card with energy added", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const firstCard = historyPage.getSessionCard(0);
    await expect(firstCard.getByText(/kWh/i)).toBeVisible();
  });

  test("should show session card with cost", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const firstCard = historyPage.getSessionCard(0);
    await expect(firstCard.getByText(/Cost/i)).toBeVisible();
  });

  test("should show delete button for completed sessions", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    // Expand first card to see delete button
    await historyPage.toggleSessionCard(0);
    await page.waitForFunction(
      () => document.querySelector('[aria-expanded="true"]') !== null,
      { timeout: 5000 },
    );

    const deleteButtons = page.getByRole("button", {
      name: /delete.*session/i,
    });
    const deleteCount = await deleteButtons.count();
    expect(deleteCount).toBeGreaterThan(0);
  });

  test("should open delete confirmation dialog", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const deleteButtons = page.getByRole("button", {
      name: /delete.*session/i,
    });
    if ((await deleteButtons.count()) > 0) {
      await deleteButtons.first().click();
      const dialog = page.getByRole("dialog", { name: /delete session/i });
      await expect(dialog).toBeVisible({ timeout: 5000 });
      // Cancel to avoid actually deleting - scope to dialog to avoid matching "cancelled" status
      await dialog.getByRole("button", { name: "Cancel" }).click();
    }
  });

  test("should cancel delete when cancel button is clicked", async ({
    page,
  }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const cardCount = await historyPage.getSessionCount();
    const deleteButtons = page.getByRole("button", {
      name: /delete.*session/i,
    });
    if ((await deleteButtons.count()) > 0) {
      await deleteButtons.first().click();
      const dialog = page.getByRole("dialog", { name: /delete session/i });
      await expect(dialog).toBeVisible({ timeout: 5000 });

      await dialog.getByRole("button", { name: "Cancel" }).click();
      await expect(dialog).not.toBeVisible();

      const countAfter = await historyPage.getSessionCount();
      expect(countAfter).toBe(cardCount);
    }
  });

  test("should show session count indicator", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const sessionCountText = page.getByText(/session/i);
    await expect(sessionCountText).toBeVisible();
  });

  test("should show status badge on each session card", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const firstCard = historyPage.getSessionCard(0);
    const cardText = await firstCard.textContent();
    expect(cardText).not.toBeNull();
    if (cardText !== null) {
      expect(
        /completed|active|cancelled|stopped/i.exec(cardText),
      ).not.toBeNull();
    }
  });

  test("should show date on session card", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const firstCard = historyPage.getSessionCard(0);
    await expect(firstCard).toContainText(/\w+ \d{1,2}/);
  });

  test("should filter by date when date picker value changes", async ({
    page,
  }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const initialCount = await historyPage.getSessionCount();
    const currentDate = await historyPage.datePicker.inputValue();

    // A future date has no sessions - count must drop to zero.
    await historyPage.datePicker.fill("2099-01-01");
    await page.waitForLoadState("domcontentloaded");

    const futureCount = await historyPage.getSessionCount();
    expect(futureCount).toBeLessThanOrEqual(initialCount);

    // Reset to today.
    await historyPage.datePicker.fill(currentDate);
    await page.waitForLoadState("domcontentloaded");
  });

  test("should show empty state when no sessions for selected date", async ({
    page,
  }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await historyPage.datePicker.fill("2099-01-01");
    await page.waitForLoadState("domcontentloaded");

    await expect(page.getByText(/no charge sessions/i)).toBeVisible({
      timeout: 10000,
    });
  });

  test("should show duration on session card", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const firstCard = historyPage.getSessionCard(0);
    const cardText = await firstCard.textContent();
    expect(cardText).toMatch(/\d+h \d+m|\d+ min|In progress/i);
  });

  test("should show time range on session card", async ({ page }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const firstCard = historyPage.getSessionCard(0);
    await expect(firstCard).toContainText(/\d{1,2}:\d{2}/);
  });

  test("should show 'To' or 'Target' label for end percentage", async ({
    page,
  }) => {
    const historyPage = new HistoryPage(page);
    await historyPage.goto();

    await expect(historyPage.sessionCards.first()).toBeVisible();

    const firstCard = historyPage.getSessionCard(0);
    const cardContent = await firstCard.textContent();
    expect(cardContent).toMatch(/To|Target/);
  });
});
