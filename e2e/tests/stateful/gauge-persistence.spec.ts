import type { Page } from "@playwright/test";
import { test, expect } from "./fixtures";
import { Vehicle } from "../helpers/auth";

/**
 * Gauge marker persistence - the most critical interaction in the app.
 *
 * These tests reproduce and guard against the historical flakiness where
 * dragging the current/target markers sometimes failed to persist (the value
 * reverted after a reload). They drive the gauge via React's fiber-stored
 * pointer handlers (see docs/TESTING.md section 5 (Browser testing))
 * because Playwright's synthetic drag does not reproduce the rAF-batched
 * pointer flow the component relies on.
 *
 * Stateful + serial: each test mutates vehicle/session percents.
 */

interface PointerLikeEvent {
  currentTarget: Element;
  clientX: number;
  clientY: number;
  pointerId: number;
  pointerType: string;
  isPrimary: boolean;
  button: number;
  buttons: number;
  type: string;
}

interface GaugeHandlers {
  onPointerDown: (e: PointerLikeEvent) => void;
  onPointerMove: (e: PointerLikeEvent) => void;
  onPointerUp: () => void;
}

// Drives a single drag gesture on the gauge from one percent to another by
// invoking the SVG's fiber-stored pointer handlers directly.
async function dragMarker(
  page: Page,
  fromPct: number,
  toPct: number,
): Promise<void> {
  await page.evaluate(
    async ({ fromPct, toPct }) => {
      const svg = document.querySelector(
        '[data-testid="speedometer-gauge-svg"]',
      );
      if (!svg) throw new Error("gauge svg not found");
      const rect = svg.getBoundingClientRect();
      const cx = rect.left + rect.width / 2;
      // The gauge viewBox is 300×280 with the arc center at (150,150).
      // 150/280 ≈ 53.6% of rendered height — NOT 50% — matching getScreenCTM logic.
      const cy = rect.top + rect.height * (150 / 280);
      // Arc radius in viewBox units is 126 out of 300 width = 0.42.
      const arcRadius = rect.width * 0.42;
      const pctToScreen = (pct: number) => {
        const angleDeg = 135 + (pct / 100) * 270;
        const angleRad = (angleDeg * Math.PI) / 180;
        return {
          x: cx + Math.cos(angleRad) * arcRadius,
          y: cy + Math.sin(angleRad) * arcRadius,
        };
      };

      const node = svg as unknown as Record<
        string,
        { memoizedProps: GaugeHandlers }
      >;
      const fiberKey = Object.keys(node).find((k) =>
        k.startsWith("__reactFiber"),
      );
      if (!fiberKey) throw new Error("react fiber not found on gauge svg");
      const handlers = node[fiberKey].memoizedProps;

      const from = pctToScreen(fromPct);
      const to = pctToScreen(toPct);
      const evt = (x: number, y: number, type: string): PointerLikeEvent => ({
        currentTarget: svg,
        clientX: x,
        clientY: y,
        pointerId: 1,
        pointerType: "mouse",
        isPrimary: true,
        button: 0,
        buttons: 1,
        type,
      });

      handlers.onPointerDown(evt(from.x, from.y, "pointerdown"));
      handlers.onPointerMove(evt(to.x, to.y, "pointermove"));
      await new Promise((r) =>
        requestAnimationFrame(() => requestAnimationFrame(r)),
      );
      handlers.onPointerUp();
      await new Promise((r) => setTimeout(r, 200));
    },
    { fromPct, toPct },
  );
}

async function selectGaragePlug(page: Page): Promise<void> {
  // "Garage Plug" is assigned to "My RM1" - select that vehicle chip.
  // The button accessible name includes the online dot: "Online My RM1" or "Offline My RM1".
  const chip = page.getByRole("button", { name: /My RM1/ }).first();
  await chip.click();
  await expect(
    chip,
    "My RM1 vehicle chip should become selected",
  ).toHaveAttribute("aria-pressed", "true");
}

test.describe.serial("Gauge Marker Persistence", () => {
  test.beforeEach(async ({ page, api }) => {
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await selectGaragePlug(page);
    // Deterministic starting point: current 20, target 80.
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    await api.patch(`/api/vehicles/${vehicles[0].id}`, {
      currentPercent: 20,
      targetPercent: 80,
    });
    await page.reload({ waitUntil: "load" });
    await expect(page.getByTestId("speedometer-gauge-svg")).toBeVisible({
      timeout: 15_000,
    });
    await selectGaragePlug(page);
  });

  test("persists a dragged target marker across a reload (idle)", async ({
    page,
    api,
  }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[0].id;

    const [resp] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes(`/api/vehicles/${vehicleId}`) &&
          r.request().method() === "PATCH",
        { timeout: 15_000 },
      ),
      dragMarker(page, 80, 65),
    ]);
    expect(resp.ok(), "target PATCH should succeed").toBe(true);

    // Backend reflects the new target.
    await expect
      .poll(
        async () =>
          (await api.getJson<Vehicle>(`/api/vehicles/${vehicleId}`))
            .targetPercent,
        { timeout: 10_000, message: "vehicle target should persist to 65" },
      )
      .toBe(65);

    // Survives a full reload.
    await page.reload({ waitUntil: "load" });
    await selectGaragePlug(page);
    await expect(
      page.getByTestId("speedometer-gauge-svg"),
      "target should still read 65% after reload",
    ).toHaveAttribute("aria-valuetext", /target 65%/, { timeout: 10_000 });
  });

  test("persists a dragged current marker across a reload (idle)", async ({
    page,
    api,
  }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[0].id;

    const [resp] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes(`/api/vehicles/${vehicleId}`) &&
          r.request().method() === "PATCH",
        { timeout: 15_000 },
      ),
      dragMarker(page, 20, 35),
    ]);
    expect(resp.ok(), "current PATCH should succeed").toBe(true);

    await expect
      .poll(
        async () =>
          (await api.getJson<Vehicle>(`/api/vehicles/${vehicleId}`))
            .currentPercent,
        { timeout: 10_000, message: "vehicle current should persist to 35" },
      )
      .toBe(35);

    await page.reload({ waitUntil: "load" });
    await selectGaragePlug(page);
    await expect(
      page.getByTestId("gauge-percent"),
      "current should still read 35% after reload",
    ).toContainText("35%", { timeout: 10_000 });
  });

  test("rapid repeated target drags persist the final value (idle)", async ({
    page,
    api,
  }) => {
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[0].id;

    // Several gestures in quick succession; only the last value should win.
    await dragMarker(page, 80, 60);
    await dragMarker(page, 60, 90);
    await dragMarker(page, 90, 75);

    await expect
      .poll(
        async () =>
          (await api.getJson<Vehicle>(`/api/vehicles/${vehicleId}`))
            .targetPercent,
        {
          timeout: 10_000,
          message: "final target after rapid drags should be 75",
        },
      )
      .toBe(75);

    await page.reload({ waitUntil: "load" });
    await selectGaragePlug(page);
    await expect(
      page.getByTestId("speedometer-gauge-svg"),
      "target should still read 75% after reload",
    ).toHaveAttribute("aria-valuetext", /target 75%/, { timeout: 10_000 });
  });

  test("updates target during a charge and clamps below current", async ({
    page,
    api,
  }) => {
    // Start + two debounced updates (each polled) + stop exceeds the default budget.
    test.setTimeout(90_000);
    const vehicles = await api.getJson<Vehicle[]>("/api/vehicles");
    const vehicleId = vehicles[0].id;

    // Start charging (synchronous activation).
    await page.getByRole("button", { name: /start charging/i }).click();
    await expect(
      page.getByRole("button", { name: /stop charging/i }),
    ).toBeVisible({ timeout: 25_000 });

    // Raise the target - persists to the active session (debounced PATCH).
    const [resp] = await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes("/api/charge-sessions") &&
          r.request().method() === "PATCH",
        { timeout: 15_000 },
      ),
      dragMarker(page, 80, 90),
    ]);
    expect(resp.ok(), "charging target PATCH should succeed").toBe(true);
    await expect
      .poll(
        async () => {
          const s = await api.getSession(
            `/api/charge-sessions?vehicleId=${vehicleId}`,
          );
          return s?.targetPercent ?? null;
        },
        { timeout: 10_000, message: "charging target should update to 90" },
      )
      .toBe(90);

    // Dragging below the current marker (20%) clamps to the current value.
    await dragMarker(page, 90, 10);
    await expect
      .poll(
        async () => {
          const s = await api.getSession(
            `/api/charge-sessions?vehicleId=${vehicleId}`,
          );
          return s?.targetPercent ?? null;
        },
        {
          timeout: 10_000,
          message: "charging target must not drop below current",
        },
      )
      .toBeGreaterThanOrEqual(20);

    // Clean up via API: clamping the target to ≈current can leave the UI in a
    // "charged" edge state rather than "Ready", so stop the session directly.
    await api
      .patch(`/api/charge-sessions?vehicleId=${vehicleId}`, {
        status: "stopped",
      })
      .catch(() => undefined);
  });
});
