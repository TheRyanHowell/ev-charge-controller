import { customRenderHook, act, waitFor } from "@/test-utils";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useMemo } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import { useHistoryDelete } from "./useHistoryDelete";
import { useHistorySessions } from "./useHistorySessions";
import { useHistoryVehicles } from "./useHistoryVehicles";

const mockFetch = vi.fn();

function TestWrapper({ children }: { children: React.ReactNode }) {
  const client = useMemo(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { retry: false },
          mutations: { retry: false },
        },
      }),
    [],
  );
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  mockFetch.mockReset();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useHistoryVehicles", () => {
  it("returns empty array while loading", () => {
    mockFetch.mockImplementation(() => new Promise(() => {}));

    const { result } = customRenderHook(() => useHistoryVehicles(), {
      wrapper: TestWrapper,
    });

    expect(result.current.vehicles).toEqual([]);
    expect(result.current.loading).toBe(true);
  });

  it("fetches vehicles on mount", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => [
        {
          id: "v1",
          name: "Car A",
          capacityKwh: 3.8,
          chargerOutputW: 1200,
          chargingEfficiency: 0.8,
          rangeMinMi: 100,
          rangeMaxMi: 150,
        },
      ],
    });

    const { result } = customRenderHook(() => useHistoryVehicles(), {
      wrapper: TestWrapper,
    });

    await waitFor(() => {
      expect(result.current.vehicles).toHaveLength(1);
    });

    expect(result.current.vehicles[0]?.name).toBe("Car A");
    expect(result.current.loading).toBe(false);
  });

  it("sets error on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = customRenderHook(() => useHistoryVehicles(), {
      wrapper: TestWrapper,
    });

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });
  });

  it("sets error on non-200 response", async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 500 });

    const { result } = customRenderHook(() => useHistoryVehicles(), {
      wrapper: TestWrapper,
    });

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });
  });
});

describe("useHistorySessions", () => {
  it("returns undefined sessions while loading", () => {
    mockFetch.mockImplementation(() => new Promise(() => {}));

    const { result } = customRenderHook(
      () => useHistorySessions(null, "2025-01-01"),
      {
        wrapper: TestWrapper,
      },
    );

    expect(result.current.sessions).toBeUndefined();
    expect(result.current.loading).toBe(true);
  });

  it("fetches sessions with correct params", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => [
        {
          id: "s1",
          vehicleId: "v1",
          createdAt: "2025-01-01T10:00:00Z",
          startKwh: 1,
          endKwh: 3,
          startPercent: 20,
          targetKwh: 3.8,
          targetPercent: 100,
          status: "completed",
        },
      ],
    });

    const { result } = customRenderHook(
      () => useHistorySessions("v1", "2025-01-01"),
      {
        wrapper: TestWrapper,
      },
    );

    await waitFor(() => {
      expect(result.current.sessions).toHaveLength(1);
    });

    const fetchCall = (mockFetch.mock.calls[0]?.[0] ?? "") as string;
    expect(fetchCall).toContain("vehicleId=v1");
    expect(fetchCall).toContain("date=2025-01-01");
    expect(fetchCall).toContain("limit=50");
    expect(fetchCall).toContain("offset=0");
  });

  it("omits vehicleId param when null", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => [],
    });

    const { result } = customRenderHook(
      () => useHistorySessions(null, "2025-01-01"),
      {
        wrapper: TestWrapper,
      },
    );

    await waitFor(() => {
      expect(result.current.sessions).toEqual([]);
    });

    const fetchCall = (mockFetch.mock.calls[0]?.[0] ?? "") as string;
    expect(fetchCall).not.toContain("vehicleId");
    expect(fetchCall).toContain("date=2025-01-01");
  });

  it("returns error on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = customRenderHook(
      () => useHistorySessions(null, "2025-01-01"),
      {
        wrapper: TestWrapper,
      },
    );

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
    });
  });

  it("returns hasMore true when page is full", async () => {
    const page1 = Array.from({ length: 50 }, (_, i) => ({
      id: `s${i}`,
      vehicleId: "v1",
      createdAt: "2025-01-01T10:00:00Z",
      startKwh: 1,
      endKwh: 3,
      startPercent: 20,
      targetKwh: 3.8,
      targetPercent: 100,
      status: "completed",
    }));

    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => page1,
    });

    const { result } = customRenderHook(
      () => useHistorySessions(null, "2025-01-01"),
      {
        wrapper: TestWrapper,
      },
    );

    await waitFor(() => {
      expect(result.current.sessions).toHaveLength(50);
    });

    expect(result.current.hasMore).toBe(true);
  });

  it("returns hasMore false when page is not full", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => [
        {
          id: "s1",
          vehicleId: "v1",
          createdAt: "2025-01-01T10:00:00Z",
          startKwh: 1,
          endKwh: 3,
          startPercent: 20,
          targetKwh: 3.8,
          targetPercent: 100,
          status: "completed",
        },
      ],
    });

    const { result } = customRenderHook(
      () => useHistorySessions(null, "2025-01-01"),
      {
        wrapper: TestWrapper,
      },
    );

    await waitFor(() => {
      expect(result.current.sessions).toHaveLength(1);
    });

    expect(result.current.hasMore).toBe(false);
  });

  it("loadMore fetches next page and appends sessions", async () => {
    const page1 = Array.from({ length: 50 }, (_, i) => ({
      id: `p1-s${i}`,
      vehicleId: "v1",
      createdAt: "2025-01-01T10:00:00Z",
      startKwh: 1,
      endKwh: 3,
      startPercent: 20,
      targetKwh: 3.8,
      targetPercent: 100,
      status: "completed",
    }));

    mockFetch.mockImplementation((url: string) => {
      const params = new URLSearchParams((url as string).split("?")[1] || "");
      const offset = parseInt(params.get("offset") || "0");
      if (offset > 0) {
        return Promise.resolve({
          ok: true,
          json: async () => [{ ...page1[0], id: "page2-s1" }],
        });
      }
      return Promise.resolve({ ok: true, json: async () => page1 });
    });

    const { result } = customRenderHook(
      () => useHistorySessions(null, "2025-01-01"),
      {
        wrapper: TestWrapper,
      },
    );

    await waitFor(() => {
      expect(result.current.sessions).toHaveLength(50);
    });

    await act(async () => {
      result.current.loadMore();
    });

    await waitFor(() => {
      expect(result.current.sessions).toHaveLength(51);
    });
  });

  it("loadMore does nothing when hasMore is false", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => [],
    });

    const { result } = customRenderHook(
      () => useHistorySessions(null, "2025-01-01"),
      {
        wrapper: TestWrapper,
      },
    );

    await waitFor(() => {
      expect(result.current.hasMore).toBe(false);
    });

    const callCount = mockFetch.mock.calls.length;

    await act(async () => {
      result.current.loadMore();
    });

    expect(mockFetch.mock.calls.length).toBe(callCount);
  });

  it("isFetchingNextPage is true during loadMore", async () => {
    const page1 = Array.from({ length: 50 }, (_, i) => ({
      id: `p1-s${i}`,
      vehicleId: "v1",
      createdAt: "2025-01-01T10:00:00Z",
      startKwh: 1,
      endKwh: 3,
      startPercent: 20,
      targetKwh: 3.8,
      targetPercent: 100,
      status: "completed",
    }));

    let resolveNextPage: (value: unknown) => void;
    const nextPagePromise = new Promise((resolve) => {
      resolveNextPage = resolve;
    });

    mockFetch.mockImplementation((url: string) => {
      const params = new URLSearchParams((url as string).split("?")[1] || "");
      const offset = parseInt(params.get("offset") || "0");
      if (offset > 0) {
        return nextPagePromise.then(() => ({
          ok: true,
          json: async () => [{ ...page1[0], id: "page2-s1" }],
        }));
      }
      return Promise.resolve({ ok: true, json: async () => page1 });
    });

    const { result } = customRenderHook(
      () => useHistorySessions(null, "2025-01-01"),
      {
        wrapper: TestWrapper,
      },
    );

    await waitFor(() => {
      expect(result.current.sessions).toHaveLength(50);
    });

    await act(async () => {
      result.current.loadMore();
    });

    await waitFor(() => {
      expect(result.current.isFetchingNextPage).toBe(true);
    });

    await act(async () => {
      resolveNextPage?.({});
    });

    await waitFor(() => {
      expect(result.current.isFetchingNextPage).toBe(false);
    });
  });
});

describe("useHistoryDelete", () => {
  it("returns isDeleting false initially", () => {
    const { result } = customRenderHook(() => useHistoryDelete(), {
      wrapper: TestWrapper,
    });
    expect(result.current.isDeleting).toBe(false);
  });

  it("calls DELETE endpoint on deleteSession", async () => {
    mockFetch.mockResolvedValue({ ok: true, status: 200 });

    const { result } = customRenderHook(() => useHistoryDelete(), {
      wrapper: TestWrapper,
    });

    let deleteResult: boolean | undefined;
    await act(async () => {
      deleteResult = await result.current.deleteSession("s1");
    });

    expect(deleteResult).toBe(true);
    const call = mockFetch.mock.calls[0];
    expect(call?.[0]).toContain("/api/charge-sessions/s1");
    expect(call?.[1]?.method).toBe("DELETE");
  });

  it("returns false on network error", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = customRenderHook(() => useHistoryDelete(), {
      wrapper: TestWrapper,
    });

    let deleteResult: boolean | undefined;
    await act(async () => {
      deleteResult = await result.current.deleteSession("s1");
    });

    expect(deleteResult).toBe(false);
  });

  it("returns false on non-ok response", async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 500 });

    const { result } = customRenderHook(() => useHistoryDelete(), {
      wrapper: TestWrapper,
    });

    let deleteResult: boolean | undefined;
    await act(async () => {
      deleteResult = await result.current.deleteSession("s1");
    });

    expect(deleteResult).toBe(false);
  });

  it("succeeds without error on valid delete", async () => {
    mockFetch.mockResolvedValue({ ok: true, status: 200 });

    const { result } = customRenderHook(() => useHistoryDelete(), {
      wrapper: TestWrapper,
    });

    let deleteResult: boolean | undefined;
    await act(async () => {
      deleteResult = await result.current.deleteSession("s1");
    });

    expect(deleteResult).toBe(true);
    expect(result.current.isDeleting).toBe(false);
  });
});
