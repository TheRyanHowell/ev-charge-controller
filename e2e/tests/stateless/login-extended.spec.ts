import { test, expect } from "./fixtures";

/**
 * Login Extended tests cover:
 * - Login page form fields and labels
 * - Form accessibility
 * - HTML5 validation behavior
 *
 * Note: These tests use an isolated browser context without storage state
 * to simulate an unauthenticated user.
 */
test.describe("Login Extended", () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.waitForURL("/login");
    await page.locator("#email").waitFor({ state: "visible" });
  });

  test("should show login page title", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "EV Charge Controller" }),
    ).toBeVisible();
  });

  test("should show email and password fields", async ({ page }) => {
    await expect(page.locator("#email")).toBeVisible();
    await expect(page.locator("#password")).toBeVisible();
  });

  test("should show sign in button", async ({ page }) => {
    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  });

  test("should have submit button enabled (HTML5 validation handles empty fields)", async ({
    page,
  }) => {
    // The button is always enabled; HTML5 `required` attribute prevents
    // submission when fields are empty
    await expect(page.getByRole("button", { name: "Sign in" })).toBeEnabled();
  });

  test("should fill both fields and have submit button enabled", async ({
    page,
  }) => {
    await page.locator("#email").fill("test@example.com");
    await page.locator("#password").fill("password123");

    // Verify fields have values
    await expect(page.locator("#email")).toHaveValue("test@example.com");
    await expect(page.locator("#password")).toHaveValue("password123");

    // Button should be enabled (ready for submission)
    await expect(page.getByRole("button", { name: "Sign in" })).toBeEnabled();
  });

  test("should have proper form accessibility attributes", async ({ page }) => {
    // Labels should be associated with inputs
    const emailLabel = page.getByText("Email address");
    const passwordLabel = page.getByText("Password");
    await expect(emailLabel).toBeVisible();
    await expect(passwordLabel).toBeVisible();

    // Inputs should have autocomplete attributes
    await expect(page.locator("#email")).toHaveAttribute(
      "autocomplete",
      "email",
    );
    await expect(page.locator("#password")).toHaveAttribute(
      "autocomplete",
      "current-password",
    );

    // Inputs should have required attribute (HTML5 validation)
    await expect(page.locator("#email")).toHaveAttribute("required");
    await expect(page.locator("#password")).toHaveAttribute("required");
  });
});
