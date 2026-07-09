import AxeBuilder from "@axe-core/playwright";
import type { Page } from "@playwright/test";

import { VehiclesPage } from "../pages/vehiclesPage";
import { test, expect } from "./fixtures";

type Theme = "light" | "dark";

// Explicit tags rather than relying on axe defaults, matching the WCAG level
// this project targets (AA), plus WCAG 2.1's additional AA success criteria.
const WCAG_TAGS = ["wcag2a", "wcag2aa", "wcag21aa"];

// Sets the theme deterministically (bypassing OS/CI-runner color scheme)
// via the same localStorage key themeStore.ts reads on boot, then reloads
// so the pre-hydration inline script in layout.tsx applies it.
async function setTheme(page: Page, theme: Theme) {
  await page.evaluate((t) => {
    localStorage.setItem("theme", t);
  }, theme);
  await page.reload({ waitUntil: "domcontentloaded" });
}

async function scanForViolations(page: Page, target: string) {
  const results = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze();

  expect(
    results.violations,
    `${target} has WCAG violations:\n${JSON.stringify(results.violations, null, 2)}`,
  ).toEqual([]);
}

const THEMES: Theme[] = ["light", "dark"];

test.describe("Accessibility (axe)", () => {
  for (const theme of THEMES) {
    test.describe(`${theme} mode`, () => {
      test("dashboard has no WCAG violations", async ({ page }) => {
        await setTheme(page, theme);
        await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
          timeout: 15_000,
        });
        await page.waitForLoadState("load");

        await scanForViolations(page, `Dashboard (${theme} mode)`);
      });

      test("vehicles list has no WCAG violations", async ({ page }) => {
        const vehiclesPage = new VehiclesPage(page);
        await vehiclesPage.goto();
        await setTheme(page, theme);
        await expect(vehiclesPage.pageTitle).toBeVisible({ timeout: 15_000 });

        await scanForViolations(page, `Vehicles list (${theme} mode)`);
      });

      test("vehicle detail has no WCAG violations", async ({ page }) => {
        const vehiclesPage = new VehiclesPage(page);
        await vehiclesPage.goto();
        await setTheme(page, theme);
        await expect(vehiclesPage.pageTitle).toBeVisible({ timeout: 15_000 });

        // "My RM2" is an unambiguous seeded vehicle name (unlike "My RM1",
        // which also prefix-matches the seeded "My RM1S").
        await vehiclesPage.navigateToVehicle("My RM2");
        await page.waitForURL(/\/vehicles\/[^/]+/, { timeout: 15_000 });

        await scanForViolations(page, `Vehicle detail (${theme} mode)`);
      });

      test("history has no WCAG violations", async ({ page }) => {
        await page.goto("/history", { waitUntil: "domcontentloaded" });
        await setTheme(page, theme);
        await expect(
          page.getByRole("heading", { name: /charge history/i }),
        ).toBeVisible({ timeout: 15_000 });

        await scanForViolations(page, `History (${theme} mode)`);
      });

      test("login has no WCAG violations", async ({ page }) => {
        // The stateless fixture's page already carries an authenticated
        // storageState; clear cookies to reach the unauthenticated login page.
        await page.context().clearCookies();
        await page.goto("/login", { waitUntil: "domcontentloaded" });
        await setTheme(page, theme);
        await expect(
          page.getByRole("heading", { name: "EV Charge Controller" }),
        ).toBeVisible({ timeout: 15_000 });

        await scanForViolations(page, `Login (${theme} mode)`);
      });

      test("settings dialog has no WCAG violations", async ({ page }) => {
        await setTheme(page, theme);
        await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
          timeout: 15_000,
        });

        await page.getByRole("button", { name: "Open settings" }).click();
        await expect(page.locator("dialog[open]")).toBeVisible({
          timeout: 10_000,
        });

        await scanForViolations(page, `Settings dialog (${theme} mode)`);
      });
    });
  }
});
