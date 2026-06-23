import type { Vehicle } from "@/lib/schemas";

import { renderHook, act } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useElapsedEta } from "./useElapsedEta";

// Isolate the timing/anchoring logic from the ETA physics model.
vi.mock("@/utils/eta", () => ({
  calculateETA: vi.fn(() => 60), // 60 minutes
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

  it("ticks elapsed and decrements remaining while charging", () => {
    const start = Date.now();
    const { result } = renderHook(() =>
      useElapsedEta({
        status: "charging",
        sessionStartTime: start,
        currentPercent: 20,
        targetPercent: 80,
        vehicle,
      }),
    );

    act(() => {
      vi.advanceTimersByTime(120_000); // 2 minutes
    });

    expect(result.current.elapsed).toBeGreaterThanOrEqual(120_000);
    // duration + remaining stays anchored to the original 60-minute estimate.
    expect(result.current.remainingSec).toBe(60 * 60 - 120);
  });
});
