import { HistoryVehicle } from "@/lib/schemas";
import { customRenderHook, waitFor } from "@/test-utils";
import { createHistoryVehicle } from "@/test/fixtures";
import { describe, it, expect, vi, beforeEach } from "vitest";

import { useHistoryVehicles } from "./useHistoryVehicles";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockVehicles: HistoryVehicle[] = [
  createHistoryVehicle({ id: "v1", capacityKwh: 3.8, chargerOutputW: 1200 }),
  createHistoryVehicle({
    id: "v2",
    name: "Test Car",
    capacityKwh: 5,
    chargerOutputW: 600,
    chargingEfficiency: 0.85,
    rangeMinMi: 120,
    rangeMaxMi: 180,
  }),
];

describe("useHistoryVehicles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("initializes with SSR data without fetching", () => {
    const { result } = customRenderHook(() =>
      useHistoryVehicles({ initialVehicles: mockVehicles }),
    );

    expect(result.current.loading).toBe(false);
    expect(result.current.vehicles).toHaveLength(2);
    expect(result.current.vehicles[0]?.name).toBe("Maeving RM1");
    expect(result.current.error).toBeNull();
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("fetches when no SSR data provided", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => mockVehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = customRenderHook(() => useHistoryVehicles());

    expect(result.current.loading).toBe(true);
    expect(result.current.vehicles).toHaveLength(0);

    await waitFor(
      () => {
        expect(result.current.vehicles).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    expect(result.current.vehicles[0]?.name).toBe("Maeving RM1");
  });

  it("handles empty SSR vehicle list", () => {
    const { result } = customRenderHook(() =>
      useHistoryVehicles({ initialVehicles: [] }),
    );

    expect(result.current.loading).toBe(false);
    expect(result.current.vehicles).toHaveLength(0);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("handles API failure without SSR data", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = customRenderHook(() => useHistoryVehicles());

    await waitFor(
      () => {
        expect(result.current.error).toBe("Failed to load vehicle list");
      },
      { timeout: 1000 },
    );
  });
});
