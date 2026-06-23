import { test, expect } from "./fixtures";

test.describe("Responsive Layout", () => {
  test("gauge is visible on mobile viewport", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 375, height: 667 },
      storageState: "storage-states/chromium.json",
    });
    const page = await context.newPage();
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10000,
    });
    await context.close();
  });

  test("gauge is visible on tablet viewport", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 768, height: 1024 },
      storageState: "storage-states/chromium.json",
    });
    const page = await context.newPage();
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10000,
    });
    await context.close();
  });

  test("gauge is visible on desktop viewport", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 1920, height: 1080 },
      storageState: "storage-states/chromium.json",
    });
    const page = await context.newPage();
    await page.goto("/dashboard");
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 10000,
    });
    await context.close();
  });

  test("header links visible on mobile", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 375, height: 667 },
      storageState: "storage-states/chromium.json",
    });
    const page = await context.newPage();
    await page.goto("/dashboard");
    await expect(
      page.getByRole("link", { name: "View charge history" }),
    ).toBeVisible({
      timeout: 10000,
    });
    await expect(
      page.getByRole("link", { name: "View vehicles" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Open settings" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Log out" })).toBeVisible();
    await context.close();
  });

  test("header links visible on desktop", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 1920, height: 1080 },
      storageState: "storage-states/chromium.json",
    });
    const page = await context.newPage();
    await page.goto("/dashboard");
    await expect(
      page.getByRole("link", { name: "View charge history" }),
    ).toBeVisible({
      timeout: 10000,
    });
    await expect(
      page.getByRole("link", { name: "View vehicles" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Open settings" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Log out" })).toBeVisible();
    await context.close();
  });

  test("no horizontal scroll on mobile viewport", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 375, height: 667 },
      storageState: "storage-states/chromium.json",
    });
    const page = await context.newPage();
    await page.goto("/dashboard");
    await page.waitForLoadState("domcontentloaded");

    // Allow small tolerance for scrollbar width (~21px)
    const pageWidth = await page.evaluate(
      () => document.documentElement.scrollWidth,
    );
    expect(pageWidth).toBeLessThanOrEqual(400);
    await context.close();
  });

  test("no horizontal scroll on desktop viewport", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 1920, height: 1080 },
      storageState: "storage-states/chromium.json",
    });
    const page = await context.newPage();
    await page.goto("/dashboard");
    await page.waitForLoadState("domcontentloaded");

    const pageWidth = await page.evaluate(
      () => document.documentElement.scrollWidth,
    );
    expect(pageWidth).toBeLessThanOrEqual(1920);
    await context.close();
  });

  test("heading visible on all viewports", async ({ browser }) => {
    for (const viewport of [
      { width: 375, height: 667 },
      { width: 768, height: 1024 },
      { width: 1920, height: 1080 },
    ]) {
      const context = await browser.newContext({
        viewport,
        storageState: "storage-states/chromium.json",
      });
      const page = await context.newPage();
      await page.goto("/dashboard");
      await expect(
        page.getByRole("heading", { name: "EV Charge Controller" }),
      ).toBeVisible({
        timeout: 10000,
      });
      await context.close();
    }
  });
});
