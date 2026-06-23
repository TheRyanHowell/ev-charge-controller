import { HistoryChargeSession } from "@/lib/schemas";
import { customRenderHook, waitFor } from "@/test-utils";
import { createHistorySession } from "@/test/fixtures";
import { describe, it, expect, vi, beforeEach } from "vitest";

import { useHistorySessions } from "./useHistorySessions";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const mockSessions: HistoryChargeSession[] = [
  createHistorySession({
    id: "s1",
    vehicleId: "v1",
    createdAt: "2024-01-15T10:00:00Z",
    endedAt: "2024-01-15T12:30:00Z",
    startKwh: 0.76,
    endKwh: 3.04,
    targetKwh: 3.8,
    startPercent: 20,
    endPercent: 80,
    targetPercent: 100,
    status: "completed",
    totalBatteryKwh: 2.85,
  }),
  createHistorySession({
    id: "s2",
    vehicleId: "v1",
    createdAt: "2024-01-15T14:00:00Z",
    endedAt: "2024-01-15T15:30:00Z",
    startKwh: 1.0,
    endKwh: 2.5,
    targetKwh: 3.8,
    startPercent: 25,
    endPercent: 65,
    targetPercent: 100,
    status: "completed",
    totalBatteryKwh: 1.5,
  }),
];

describe("useHistorySessions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("initializes with SSR data without fetching", () => {
    const { result } = customRenderHook(() =>
      useHistorySessions("v1", "2024-01-15", {
        initialSessions: mockSessions,
      }),
    );

    expect(result.current.loading).toBe(false);
    expect(result.current.sessions).toHaveLength(2);
    expect(result.current.sessions?.[0]?.id).toBe("s1");
    expect(result.current.error).toBeNull();
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("fetches when no SSR data provided", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/history")) {
        return Promise.resolve({
          ok: true,
          json: async () => mockSessions,
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = customRenderHook(() =>
      useHistorySessions("v1", "2024-01-15"),
    );

    expect(result.current.loading).toBe(true);
    expect(result.current.sessions).toBeUndefined();

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    expect(result.current.sessions?.[0]?.id).toBe("s1");
  });

  it("handles empty SSR sessions list", () => {
    const { result } = customRenderHook(() =>
      useHistorySessions("v1", "2024-01-15", {
        initialSessions: [],
      }),
    );

    expect(result.current.loading).toBe(false);
    expect(result.current.sessions).toHaveLength(0);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("handles API failure without SSR data", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = customRenderHook(() =>
      useHistorySessions("v1", "2024-01-15"),
    );

    await waitFor(
      () => {
        expect(result.current.error).toBe(
          "Unable to connect to the API server. Is the backend running?",
        );
      },
      { timeout: 1000 },
    );
  });

  it("fetches fresh data when date changes instead of replaying initial sessions", async () => {
    const newSessions: HistoryChargeSession[] = [
      {
        id: "s3",
        vehicleId: "v1",
        createdAt: "2024-01-16T10:00:00Z",
        endedAt: "2024-01-16T12:30:00Z",
        startKwh: 0.5,
        endKwh: 2.0,
        targetKwh: 3.8,
        startPercent: 15,
        endPercent: 70,
        targetPercent: 100,
        status: "completed",
        totalBatteryKwh: 1.5,
      },
    ];

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/history") && url.includes("date=2024-01-16")) {
        return Promise.resolve({
          ok: true,
          json: async () => newSessions,
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result, rerender } = customRenderHook(
      (props) =>
        useHistorySessions(props.vehicleId, props.date, {
          initialSessions: props.initialSessions,
        }),
      {
        initialProps: {
          vehicleId: "v1",
          date: "2024-01-15",
          initialSessions: mockSessions,
        },
      },
    );

    // Initial: uses SSR data, no fetch
    expect(result.current.sessions).toHaveLength(2);
    expect(result.current.sessions?.[0]?.id).toBe("s1");
    expect(mockFetch).not.toHaveBeenCalled();

    // Change date
    rerender({
      vehicleId: "v1",
      date: "2024-01-16",
      initialSessions: mockSessions,
    });

    // Should fetch new data, not replay initial sessions
    await waitFor(
      () => {
        expect(mockFetch).toHaveBeenCalled();
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    expect(result.current.sessions?.[0]?.id).toBe("s3");
  });

  it("fetches fresh data when vehicle filter changes instead of replaying initial sessions", async () => {
    const newSessions: HistoryChargeSession[] = [
      {
        id: "s4",
        vehicleId: "v2",
        createdAt: "2024-01-15T11:00:00Z",
        endedAt: "2024-01-15T13:30:00Z",
        startKwh: 0.5,
        endKwh: 2.0,
        targetKwh: 3.8,
        startPercent: 15,
        endPercent: 70,
        targetPercent: 100,
        status: "completed",
        totalBatteryKwh: 1.5,
      },
    ];

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/history") && url.includes("vehicleId=v2")) {
        return Promise.resolve({
          ok: true,
          json: async () => newSessions,
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result, rerender } = customRenderHook(
      (props) =>
        useHistorySessions(props.vehicleId, props.date, {
          initialSessions: props.initialSessions,
        }),
      {
        initialProps: {
          vehicleId: "v1",
          date: "2024-01-15",
          initialSessions: mockSessions,
        },
      },
    );

    expect(result.current.sessions).toHaveLength(2);
    expect(mockFetch).not.toHaveBeenCalled();

    // Change vehicle filter
    rerender({
      vehicleId: "v2",
      date: "2024-01-15",
      initialSessions: mockSessions,
    });

    await waitFor(
      () => {
        expect(mockFetch).toHaveBeenCalled();
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    expect(result.current.sessions?.[0]?.id).toBe("s4");
  });
});
