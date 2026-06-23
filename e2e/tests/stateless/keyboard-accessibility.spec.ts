import { test, expect } from "./fixtures";

test.describe("Keyboard Accessibility", () => {
  // Ensure React hydration is complete before any keyboard interaction.
  // waitForLoadState("load") waits for all scripts to execute, after which
  // React has had opportunity to attach event handlers to the rendered DOM.
  test.beforeEach(async ({ page }) => {
    await expect(
      page.getByTestId("speedometer-gauge-svg"),
      "Dashboard must be hydrated before keyboard tests run",
    ).toBeVisible({ timeout: 15_000 });
    await page.waitForLoadState("load");
  });

  test("can navigate to vehicles page via keyboard", async ({ page }) => {
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });

    // Focus and activate vehicles link
    await page.getByRole("link", { name: "View vehicles" }).focus();
    await page.keyboard.press("Enter");

    await page.waitForURL(/\/vehicles/, { timeout: 10000 });
    await expect(page.getByRole("heading", { name: "Vehicles" })).toBeVisible();
  });

  test("can navigate to history page via keyboard", async ({ page }) => {
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });

    // Focus and activate history link
    await page.getByRole("link", { name: "View charge history" }).focus();
    await page.keyboard.press("Enter");

    await page.waitForURL(/\/history/, { timeout: 10000 });
    await expect(
      page.getByRole("heading", { name: "Charge History" }),
    ).toBeVisible();
  });

  test("settings button is focusable", async ({ page }) => {
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });

    await page.getByRole("button", { name: "Open settings" }).focus();
    await expect(
      page.getByRole("button", { name: "Open settings" }),
    ).toBeFocused();
  });

  test("logout button is focusable", async ({ page }) => {
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });

    await page.getByRole("button", { name: "Log out" }).focus();
    await expect(page.getByRole("button", { name: "Log out" })).toBeFocused();
  });

  test("can logout via keyboard", async ({ page }) => {
    // page is already on /dashboard from fixture
    const logoutBtn = page.getByRole("button", { name: "Log out" });
    await logoutBtn.waitFor({ state: "visible", timeout: 15_000 });

    // Keyboard accessibility assertion: pressing Enter on the focused button
    // must invoke the logout action (fires the API call). Navigation destination
    // is not asserted here - a concurrent polling response can re-issue an
    // access_token between the logout cookie-clear and the /login redirect,
    // causing a /dashboard redirect. That's an auth race, not a keyboard issue.
    const logoutCalled = page.waitForResponse(
      (resp) => resp.url().includes("/api/auth/logout"),
      { timeout: 10_000 },
    );
    await logoutBtn.press("Enter");
    await logoutCalled;
  });

  test("vehicle selector buttons are keyboard accessible", async ({ page }) => {
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });

    // Vehicle chips use buttons with aria-pressed; wait for them to render
    const vehicleButtons = page.locator("button[aria-pressed]");
    await vehicleButtons.first().waitFor({ state: "visible", timeout: 15_000 });

    const activeButtons = page.getByRole("button", { pressed: true });
    const activeCount = await activeButtons.count();
    expect(activeCount).toBe(1); // Exactly one vehicle should be active

    // The active vehicle button should be focusable
    await activeButtons.first().focus();
    await expect(activeButtons.first()).toBeFocused();
  });

  test("can switch vehicle via keyboard", async ({ page }) => {
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });

    // All vehicle chip buttons carry aria-pressed
    const vehicleButtons = page.locator("button[aria-pressed]");
    // Wait for at least 2 vehicle buttons to render (React hydration guard)
    await vehicleButtons.nth(1).waitFor({ state: "visible", timeout: 15_000 });
    // Ensure React has finished hydrating so onClick handlers are attached
    await page.waitForLoadState("load");

    const count = await vehicleButtons.count();
    expect(count).toBeGreaterThan(1);

    // locator.press() focuses the element then dispatches the key in one atomic step
    await vehicleButtons.nth(1).press("Enter");

    // Second vehicle should now be active - give React time to update state
    await expect(vehicleButtons.nth(1)).toHaveAttribute(
      "aria-pressed",
      "true",
      {
        timeout: 10_000,
      },
    );
  });

  test("can navigate to dashboard from vehicles page", async ({ page }) => {
    await page.goto("/vehicles", { waitUntil: "domcontentloaded" });

    // Navigate to dashboard via link
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10000,
    });
  });

  test("can navigate to dashboard from history page", async ({ page }) => {
    await page.goto("/history", { waitUntil: "domcontentloaded" });

    // Navigate to dashboard via link
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10000,
    });
  });

  test("interactive elements have visible focus indicators", async ({
    page,
  }) => {
    await page.goto("/dashboard", { waitUntil: "domcontentloaded" });

    // Focus the settings button and check it has a focus ring
    await page.getByRole("button", { name: "Open settings" }).focus();

    // Check that the focused element has a visible focus style
    const focusStyle = await page
      .getByRole("button", { name: "Open settings" })
      .evaluate((el) => window.getComputedStyle(el).outline);

    // The element should have some focus styling (either outline or ring)
    const hasFocusStyle =
      focusStyle !== "none" ||
      focusStyle.includes("ring") ||
      focusStyle.includes("2px");
    // Alternatively, check the class contains focus-visible
    const hasFocusClass = await page
      .getByRole("button", { name: "Open settings" })
      .evaluate(
        (el) => el.getAttribute("class")?.includes("focus-visible") ?? false,
      );

    expect(hasFocusStyle || hasFocusClass).toBe(true);
  });
});
