import { customRenderHook as renderHook, waitFor, act } from "@/test-utils";
import { createCarbonIntensity } from "@/test/fixtures";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import { useCarbonIntensity } from "./useCarbonIntensity";

const mockFetch = vi.fn();

describe("useCarbonIntensity", () => {
  const mockIntensity = createCarbonIntensity({
    forecast: 250,
    actual: 240,
    index: "moderate",
  });

  beforeEach(() => {
    vi.stubGlobal("fetch", mockFetch);
    mockFetch.mockReset();
    mockFetch.mockImplementation(() =>
      Promise.resolve({ ok: true, json: async () => mockIntensity }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("fetches carbon intensity and returns it", async () => {
    const { result } = renderHook(() => useCarbonIntensity());

    await waitFor(() => {
      expect(result.current.carbonIntensity).toEqual(mockIntensity);
    });
  });

  it("accepts initialData", () => {
    const initial = createCarbonIntensity({
      forecast: 200,
      actual: 190,
      index: "low",
    });

    const { result } = renderHook(() => useCarbonIntensity(initial));

    expect(result.current.carbonIntensity).toEqual(initial);
  });

  it("accepts null initialData", () => {
    const { result } = renderHook(() => useCarbonIntensity(null));

    expect(result.current.carbonIntensity).toBeNull();
  });

  it("returns null when no data available", async () => {
    mockFetch.mockImplementation(() =>
      Promise.resolve({
        ok: false,
        status: 500,
        json: async () => ({ detail: "error" }),
      }),
    );

    const { result } = renderHook(() => useCarbonIntensity(null));

    expect(result.current.carbonIntensity).toBeNull();
  });

  it("handles fetch error gracefully", async () => {
    mockFetch.mockImplementation(() =>
      Promise.reject(new Error("network error")),
    );

    const { result } = renderHook(() => useCarbonIntensity());

    // Should not crash, carbonIntensity should remain null
    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.carbonIntensity).toBeNull();
  });
});
