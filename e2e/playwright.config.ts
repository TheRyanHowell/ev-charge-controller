import { defineConfig, devices } from "@playwright/test";
import path from "path";

const baseURL = process.env.PLAYWRIGHT_BASE_URL || "http://localhost:3000";
const isCI = !!process.env.CI;
const storageStatesDir = path.join(__dirname, "storage-states");

export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  forbidOnly: isCI,
  retries: isCI ? 2 : 1,
  reporter: isCI
    ? [["list"], ["html", { open: "never" }]]
    : [["list"], ["html"]],
  globalSetup: "./tests/helpers/global-setup",
  globalTimeout: 600_000,

  use: {
    baseURL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    actionTimeout: 10_000,
    navigationTimeout: 15_000,
  },

  projects: [
    // Stateless tests: run in full parallel, no per-test DB reset.
    // These run FIRST (no dependencies), so they see clean seed data.
    {
      name: "chromium-stateless",
      testDir: "./tests/stateless",
      use: {
        ...devices["Desktop Chrome"],
        storageState: path.join(storageStatesDir, "chromium.json"),
      },
    },
    {
      name: "firefox-stateless",
      testDir: "./tests/stateless",
      use: {
        ...devices["Desktop Firefox"],
        storageState: path.join(storageStatesDir, "firefox.json"),
      },
    },
    // Stateful tests: run serially, per-test DB reset.
    {
      name: "chromium-stateful",
      testDir: "./tests/stateful",
      fullyParallel: false,
      use: {
        ...devices["Desktop Chrome"],
        storageState: path.join(storageStatesDir, "chromium.json"),
      },
    },
    {
      name: "firefox-stateful",
      testDir: "./tests/stateful",
      fullyParallel: false,
      use: {
        ...devices["Desktop Firefox"],
        storageState: path.join(storageStatesDir, "firefox.json"),
      },
    },
  ],

  timeout: 30_000,
});
