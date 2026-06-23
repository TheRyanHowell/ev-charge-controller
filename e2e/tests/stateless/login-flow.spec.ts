import { test, expect } from "./fixtures";

test.describe("Login Flow", () => {
  test.beforeEach(async ({ page }) => {
    // Clear auth cookies to test login from scratch
    await page.context().clearCookies();
  });

  test("should show login page title", async ({ page }) => {
    await page.goto("/login");
    await expect(
      page.getByRole("heading", { name: "EV Charge Controller" }),
    ).toBeVisible();
    await expect(page.getByText("Sign in to your account")).toBeVisible();
  });

  test("should show email and password fields", async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByLabel("Email address")).toBeVisible();
    await expect(page.getByLabel("Password")).toBeVisible();
  });

  test("should show sign in button", async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  });

  test("should redirect to dashboard after successful login", async ({
    page,
  }) => {
    await page.goto("/login");

    await page.getByLabel("Email address").fill("test@example.com");
    await page.getByLabel("Password").fill("password123");
    await page.getByRole("button", { name: "Sign in" }).click();

    await page.waitForURL(/\/(dashboard|$)/, { timeout: 15000 });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10000,
    });
  });

  test("should show error message for invalid credentials", async ({
    page,
  }) => {
    await page.goto("/login");

    await page.getByLabel("Email address").fill("wrong@example.com");
    await page.getByLabel("Password").fill("wrongpassword");
    await page.getByRole("button", { name: "Sign in" }).click();

    // Should show an error message (use getByText to avoid Next.js route announcer conflict)
    await expect(page.getByText("invalid email or password")).toBeVisible({
      timeout: 10000,
    });
  });

  test("should submit with empty email and show server error", async ({
    page,
  }) => {
    await page.goto("/login");

    // Fill only password (form has noValidate so submission proceeds)
    await page.getByLabel("Password").fill("password123");
    await page.getByRole("button", { name: "Sign in" }).click();

    // Server should return an error for empty/invalid email
    await expect(page.getByText(/invalid email or password/i)).toBeVisible({
      timeout: 10000,
    });
  });

  test("should submit with empty password and show server error", async ({
    page,
  }) => {
    await page.goto("/login");

    // Fill only email (form has noValidate so submission proceeds)
    await page.getByLabel("Email address").fill("test@example.com");
    await page.getByRole("button", { name: "Sign in" }).click();

    // Server should return an error for empty password
    await expect(page.getByText(/invalid email or password/i)).toBeVisible({
      timeout: 10000,
    });
  });

  test("should have proper form accessibility attributes", async ({ page }) => {
    await page.goto("/login");

    // Form should have proper labels
    await expect(page.getByLabel("Email address")).toHaveAttribute(
      "type",
      "email",
    );
    await expect(page.getByLabel("Password")).toHaveAttribute(
      "type",
      "password",
    );
  });

  test("should remember email in input after failed login", async ({
    page,
  }) => {
    await page.goto("/login");

    await page.getByLabel("Email address").fill("test@example.com");
    await page.getByLabel("Password").fill("wrongpassword");
    await page.getByRole("button", { name: "Sign in" }).click();

    // Email should still be in the field (it's controlled by React state, so we need to check the ref value)
    const emailValue = await page.getByLabel("Email address").inputValue();
    expect(emailValue).toBe("test@example.com");
  });

  test("should navigate to login page via direct URL", async ({ page }) => {
    await page.goto("/login");
    await expect(
      page.getByRole("heading", { name: "EV Charge Controller" }),
    ).toBeVisible();
  });
});
