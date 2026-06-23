import { chromium, firefox } from "@playwright/test";
import path from "path";

const API_BASE_URL = process.env.E2E_API_URL ?? "http://api:8080";
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? "http://localhost:3000";
const TEST_EMAIL = process.env.E2E_TEST_EMAIL ?? "test@example.com";
const TEST_PASSWORD = process.env.E2E_TEST_PASSWORD ?? "password123";

/**
 * Global setup: authenticate once per browser and save storage state.
 *
 * Iterates over the two distinct browsers (chromium, firefox) rather than
 * config.projects, so stateless and stateful projects for the same browser
 * reuse a single auth session. The output filenames match what
 * playwright.config.ts reads: storage-states/chromium.json and
 * storage-states/firefox.json.
 */
export default async function globalSetup() {
  const storageStatesDir = path.join(__dirname, "..", "..", "storage-states");

  // Verify API is reachable before doing anything.
  const healthRes = await fetch(`${API_BASE_URL}/health`);
  if (!healthRes.ok) {
    throw new Error(
      `E2E global setup: API health check failed (${String(healthRes.status)})`,
    );
  }

  // Reset DB to deterministic seed state. Users and their refresh_tokens are
  // preserved by /api/reset, so auth cookies issued here remain valid across
  // per-test resets in the stateful suite.
  const resetRes = await fetch(`${API_BASE_URL}/api/reset`, { method: "POST" });
  if (resetRes.status === 200) {
    console.log("[global-setup] Database reset to seed state");
    // Brief settle delay - wait for plugs to come online via MQTT.
    await new Promise((r) => setTimeout(r, 2000));
  } else if (resetRes.status !== 404) {
    // 404 = dev-only endpoint unavailable (production mode) - fall through.
    const body = await resetRes.text().catch(() => "");
    throw new Error(
      `E2E global setup: reset failed (${String(resetRes.status)}) ${body}`,
    );
  }

  // Wait for the UI to be accepting connections. The container health check
  // can pass before Next.js finishes booting, so we poll with backoff.
  const UI_POLL_ATTEMPTS = 15;
  const UI_POLL_INTERVAL_MS = 2000;
  for (let i = 0; i < UI_POLL_ATTEMPTS; i++) {
    try {
      const res = await fetch(`${BASE_URL}/login`);
      if (res.ok || res.status < 500) break;
    } catch {
      if (i === UI_POLL_ATTEMPTS - 1) {
        throw new Error(
          `E2E global setup: UI did not become ready after ${String((UI_POLL_ATTEMPTS * UI_POLL_INTERVAL_MS) / 1000)}s`,
        );
      }
    }
    await new Promise((r) => setTimeout(r, UI_POLL_INTERVAL_MS));
  }

  // Authenticate once per browser. stateless + stateful projects for the same
  // browser both read the same file, so a single login covers both.
  const browsers = [
    { key: "chromium", launch: chromium },
    { key: "firefox", launch: firefox },
  ] as const;

  for (const { key, launch } of browsers) {
    const browser = await launch.launch();
    const context = await browser.newContext();
    const page = await context.newPage();

    await page.goto(`${BASE_URL}/login`);
    await page.locator("#email").waitFor({ state: "visible" });

    await page.locator("#email").fill(TEST_EMAIL);
    await page.locator("#password").fill(TEST_PASSWORD);

    const [loginResponse] = await Promise.all([
      page.waitForResponse((resp) => resp.url().includes("/api/auth/login"), {
        timeout: 15_000,
      }),
      page.getByRole("button", { name: "Sign in" }).click(),
    ]);

    if (!loginResponse.ok()) {
      throw new Error(
        `E2E global setup: login failed for ${key} (${String(loginResponse.status())})`,
      );
    }

    // Navigate to dashboard to confirm the session cookie is fully set.
    // Use domcontentloaded - dashboard polls so "load" can stall indefinitely.
    await page.goto(`${BASE_URL}/dashboard`, { waitUntil: "domcontentloaded" });
    await page.waitForURL(/\/dashboard/, { timeout: 15_000 });

    const statePath = path.join(storageStatesDir, `${key}.json`);
    await context.storageState({ path: statePath });
    console.log(`[global-setup] Authenticated as ${TEST_EMAIL} (${key})`);

    await browser.close();
  }
}
