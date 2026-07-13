import type { Vehicle } from "@/lib/schemas";

import { renderHook, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useElapsedEta } from "./useElapsedEta";

// Isolate the timing logic from the ETA physics model: 1 minute per percent
// still to charge, so the live estimate shrinks as currentPercent rises.
vi.mock("@/utils/eta", () => ({
  calculateETA: vi.fn(
    ({
      currentPercent,
      targetPercent,
    }: {
      currentPercent: number;
      targetPercent: number;
    }) =>
      targetPercent > currentPercent ? targetPercent - currentPercent : null,
  ),
}));

const vehicle = {
  id: "rm1",
  name: "Test",
  capacityKwh: 50,
  chargerOutputW: 7000,
  chargingEfficiency: 0.9,
  rangeMinMi: 100,
  rangeMaxMi: 200,
} as Vehicle;

describe("useElapsedEta", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-13T12:00:00Z"));
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("returns null estimates when no vehicle is set", () => {
    const { result } = renderHook(() =>
      useElapsedEta({
        status: "idle",
        sessionStartTime: null,
        currentPercent: 20,
        targetPercent: 80,
        vehicle: null,
      }),
    );
    expect(result.current.totalTimeMin).toBeNull();
    expect(result.current.remainingSec).toBeNull();
    expect(result.current.elapsed).toBe(0);
  });

  it("seeds totalTimeMin from the ETA and keeps elapsed at 0 when idle", () => {
    const { result } = renderHook(() =>
      useElapsedEta({
        status: "idle",
        sessionStartTime: null,
        currentPercent: 20,
        targetPercent: 80,
        vehicle,
      }),
    );
    expect(result.current.totalTimeMin).toBe(60);
    expect(result.current.elapsed).toBe(0);
    // remaining == full estimate while idle (no elapsed)
    expect(result.current.remainingSec).toBe(60 * 60);
  });

  it("ticks elapsed and tracks the live ETA while charging", () => {
    const start = Date.now();
    const { result, rerender } = renderHook(
      ({ currentPercent }: { currentPercent: number }) =>
        useElapsedEta({
          status: "charging",
          sessionStartTime: start,
          currentPercent,
          targetPercent: 80,
          vehicle,
        }),
      { initialProps: { currentPercent: 20 } },
    );

    act(() => {
      vi.advanceTimersByTime(120_000); // 2 minutes
    });

    expect(result.current.elapsed).toBeGreaterThanOrEqual(120_000);
    // Remaining reflects the live estimate from the CURRENT percent, so it
    // stays truthful even when charging is slower or faster than first quoted.
    expect(result.current.remainingSec).toBe(60 * 60);

    // Charge progresses to 50% → live estimate is now 30 minutes.
    rerender({ currentPercent: 50 });
    expect(result.current.remainingSec).toBe(30 * 60);
  });

  it("never clamps remaining to 0 while still below target (regression: slow charge past original quote)", () => {
    const start = Date.now();
    const { result, rerender } = renderHook(
      ({ currentPercent }: { currentPercent: number }) =>
        useElapsedEta({
          status: "charging",
          sessionStartTime: start,
          currentPercent,
          targetPercent: 80,
          vehicle,
        }),
      { initialProps: { currentPercent: 20 } },
    );

    // Charging ran 70 minutes - PAST the original 60-minute quote - but the
    // battery is only at 75%. The old anchored countdown showed 0 remaining
    // here; the display must instead show the honest live estimate.
    act(() => {
      vi.advanceTimersByTime(70 * 60_000);
    });
    rerender({ currentPercent: 75 });

    expect(result.current.elapsed).toBeGreaterThanOrEqual(70 * 60_000);
    expect(result.current.remainingSec).toBe(5 * 60);

    // Target completion stays consistent: session start + elapsed + remaining.
    expect(result.current.baseTime).toBe(start);
    expect(result.current.totalTimeMin).toBeCloseTo(70 + 5, 1);
  });

  it("reports 0 remaining once the target is reached but the session is still wrapping up", () => {
    const start = Date.now();
    const { result } = renderHook(() =>
      useElapsedEta({
        status: "conditioning",
        sessionStartTime: start,
        currentPercent: 100,
        targetPercent: 100,
        vehicle,
      }),
    );
    expect(result.current.remainingSec).toBe(0);
  });
});
