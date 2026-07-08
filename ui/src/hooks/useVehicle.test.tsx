import type { Vehicle } from "@/lib/schemas";

import { queryKeys } from "@/lib/queryKeys";
import { customRenderHook as renderHook, waitFor, act } from "@/test-utils";
import { createVehicle } from "@/test/fixtures";
import {
  QueryClient,
  QueryClientProvider,
  useQueryClient,
} from "@tanstack/react-query";
import { renderHook as rtlRenderHook } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

import { useVehicle } from "./useVehicle";

const mockFetch = vi.fn();
global.fetch = mockFetch;

describe("useVehicle", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    mockFetch.mockImplementation(() =>
      Promise.resolve({ ok: true, json: async () => [] }),
    );
  });

  const flushMicrotasks = () => act(async () => await Promise.resolve());

  const vehicles: Vehicle[] = [
    createVehicle({ id: "rm1" }),
    createVehicle({ id: "rm2", name: "Maeving RM2", capacityKwh: 5.46 }),
  ];

  const setup = () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });
    return renderHook(() => useVehicle());
  };

  it("starts with isLoading true while fetching vehicles", async () => {
    const { result } = setup();
    expect(result.current.isLoading).toBe(true);
    await flushMicrotasks();
  });

  it("opens settings and loads vehicles", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() => useVehicle());

    await waitFor(
      () => {
        expect(result.current.vehicles).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    await act(async () => {
      await result.current.handleOpenSettings();
    });

    expect(result.current.isSettingsOpen).toBe(true);
    expect(result.current.vehicles).toHaveLength(2);
  });

  it("closes settings modal", async () => {
    const { result } = renderHook(() => useVehicle());
    expect(result.current.isSettingsOpen).toBe(false);

    await act(async () => {
      result.current.closeSettings();
    });

    expect(result.current.isSettingsOpen).toBe(false);
  });

  it("handles fetch failure gracefully", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = renderHook(() => useVehicle());

    await waitFor(
      () => {
        expect(result.current.error).toBeTruthy();
      },
      { timeout: 1000 },
    );

    expect(result.current.vehicles).toHaveLength(0);
  });

  it("flashTempError auto-clears after timer", async () => {
    vi.useFakeTimers();

    const { result } = renderHook(() => useVehicle());

    await act(async () => {
      result.current.setTempError("Test error");
    });

    expect(result.current.tempError).toBe("Test error");

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(result.current.tempError).toBeNull();

    vi.useRealTimers();
  });

  it("updatePercents sends PATCH with percents", async () => {
    mockFetch.mockImplementation((url: string, opts?: any) => {
      if (opts?.method === "PATCH" && url.includes("/api/vehicles/")) {
        return Promise.resolve({ ok: true, status: 200 });
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() => useVehicle());

    await waitFor(
      () => {
        expect(result.current.vehicles).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    let ok = false;
    await act(async () => {
      ok = await result.current.updatePercents("rm1", 45, 80);
    });

    expect(ok).toBe(true);
    const patchCalls = mockFetch.mock.calls.filter(
      (call) => call[1] && call[1].method === "PATCH",
    );
    expect(patchCalls.length).toBeGreaterThan(0);
    const lastPatch = patchCalls.at(-1);
    if (!lastPatch) throw new Error("no patch calls");
    expect(lastPatch[1].body).toContain('"currentPercent"');
    expect(lastPatch[1].body).toContain('"targetPercent"');
  });

  it("updatePercents returns false on failure", async () => {
    mockFetch.mockImplementation((url: string, opts?: any) => {
      if (opts?.method === "PATCH" && url.includes("/api/vehicles/")) {
        return Promise.resolve({ ok: false, status: 400 });
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() => useVehicle());

    await waitFor(
      () => {
        expect(result.current.vehicles).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    let ok = true;
    await act(async () => {
      ok = await result.current.updatePercents("rm1", 45, 80);
    });

    expect(ok).toBe(false);
  });

  it("updatePercents returns false on network error", async () => {
    mockFetch.mockImplementation((url: string, opts?: any) => {
      if (opts?.method === "PATCH") {
        return Promise.reject(new Error("ECONNREFUSED"));
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() => useVehicle());

    await waitFor(
      () => {
        expect(result.current.vehicles).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    let ok = true;
    await act(async () => {
      ok = await result.current.updatePercents("rm1", 45, 80);
    });

    expect(ok).toBe(false);
  });

  it("updatePercents invalidates vehicle cache on success", async () => {
    const updatedVehicles: Vehicle[] = [
      {
        ...vehicles[0],
        id: "rm1",
        name: "Maeving RM1",
        capacityKwh: 2.026,
        chargerOutputW: 600,
        chargingEfficiency: 0.8,
        rangeMinMi: 0,
        rangeMaxMi: 0,
        currentPercent: 45,
        targetPercent: 80,
      },
      {
        ...vehicles[1],
        id: "rm2",
        name: "Maeving RM2",
        capacityKwh: 5.46,
        chargerOutputW: 600,
        chargingEfficiency: 0.8,
        rangeMinMi: 0,
        rangeMaxMi: 0,
      },
    ];
    let patchCalled = false;

    mockFetch.mockImplementation((url: string, opts?: any) => {
      if (opts?.method === "PATCH" && url.includes("/api/vehicles/")) {
        patchCalled = true;
        return Promise.resolve({ ok: true, status: 204 });
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({
          ok: true,
          json: async () => (patchCalled ? updatedVehicles : vehicles),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() => useVehicle());

    await waitFor(
      () => {
        expect(result.current.vehicles).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    await act(async () => {
      await result.current.updatePercents("rm1", 45, 80);
    });

    await waitFor(
      () => {
        const v = result.current.vehicles.find((v) => v.id === "rm1");
        expect(v?.currentPercent).toBe(45);
        expect(v?.targetPercent).toBe(80);
      },
      { timeout: 1000 },
    );
  });

  // Renders useVehicle with a QueryClient exposed to the test, so the schedule
  // cache's invalidation state can be inspected directly (mirrors the pattern in
  // useChargeActions.test.tsx, since customRenderHook's client isn't exposed).
  function renderVehicleWithClient() {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false, staleTime: 0 },
        mutations: { retry: false },
      },
    });
    function Wrapper({ children }: { children: React.ReactNode }) {
      return (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      );
    }
    const rendered = rtlRenderHook(
      () => {
        const vehicle = useVehicle();
        const client = useQueryClient();
        return { ...vehicle, queryClient: client };
      },
      { wrapper: Wrapper },
    );
    return { ...rendered, queryClient };
  }

  it("updatePercents invalidates the plug's schedule cache when a plugId is given", async () => {
    mockFetch.mockImplementation((url: string, opts?: any) => {
      if (opts?.method === "PATCH" && url.includes("/api/vehicles/")) {
        return Promise.resolve({ ok: true, status: 200 });
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderVehicleWithClient();
    const scheduleKey = queryKeys.plugs.schedule("plug-1");

    await waitFor(() => {
      expect(result.current.vehicles).toHaveLength(2);
    });

    act(() => {
      result.current.queryClient.setQueryData(scheduleKey, {
        id: "sched1",
        type: "carbon_aware",
        time: "01:00",
        enabled: true,
      });
    });

    await act(async () => {
      await result.current.updatePercents("rm1", 45, 80, "plug-1");
    });

    expect(
      result.current.queryClient.getQueryState(scheduleKey)?.isInvalidated,
    ).toBe(true);
  });

  it("updatePercents does not touch any schedule cache when plugId is omitted", async () => {
    mockFetch.mockImplementation((url: string, opts?: any) => {
      if (opts?.method === "PATCH" && url.includes("/api/vehicles/")) {
        return Promise.resolve({ ok: true, status: 200 });
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderVehicleWithClient();
    const scheduleKey = queryKeys.plugs.schedule("plug-1");

    await waitFor(() => {
      expect(result.current.vehicles).toHaveLength(2);
    });

    act(() => {
      result.current.queryClient.setQueryData(scheduleKey, {
        id: "sched1",
        type: "carbon_aware",
        time: "01:00",
        enabled: true,
      });
    });

    await act(async () => {
      await result.current.updatePercents("rm1", 45, 80);
    });

    expect(
      result.current.queryClient.getQueryState(scheduleKey)?.isInvalidated,
    ).toBeFalsy();
  });

  it("updatePercents optimistically reflects new percents before the request resolves", async () => {
    let resolvePatch: (v: unknown) => void = () => {};
    const patchPromise = new Promise((r) => {
      resolvePatch = r;
    });
    mockFetch.mockImplementation((url: string, opts?: any) => {
      if (opts?.method === "PATCH" && url.includes("/api/vehicles/")) {
        return patchPromise;
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useVehicle({
        initialVehicles: vehicles,
        initialDataUpdatedAt: Date.now(),
      }),
    );

    // Fire without awaiting so we observe the cache between onMutate and settle.
    act(() => {
      void result.current.updatePercents("rm1", 55, 95);
    });

    await waitFor(() => {
      const v = result.current.vehicles.find((x) => x.id === "rm1");
      expect(v?.currentPercent).toBe(55);
      expect(v?.targetPercent).toBe(95);
    });

    await act(async () => {
      resolvePatch({ ok: true, status: 204 });
      await Promise.resolve();
    });
  });

  it("updatePercents rolls back the cache and flashes an error on failure", async () => {
    mockFetch.mockImplementation((url: string, opts?: any) => {
      if (opts?.method === "PATCH" && url.includes("/api/vehicles/")) {
        return Promise.resolve({ ok: false, status: 400 });
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useVehicle({
        initialVehicles: vehicles,
        initialDataUpdatedAt: Date.now(),
      }),
    );

    await act(async () => {
      await result.current.updatePercents("rm1", 55, 95);
    });

    await waitFor(() => {
      expect(result.current.tempError).toBeTruthy();
    });
    const v = result.current.vehicles.find((x) => x.id === "rm1");
    expect(v?.currentPercent).not.toBe(55);
  });

  it("initializes with SSR data without fetching", async () => {
    const ssrVehicles = [vehicles[0] as Vehicle];

    const { result } = renderHook(() =>
      useVehicle({
        initialVehicles: ssrVehicles,
      }),
    );

    expect(result.current.isLoading).toBe(false);
    expect(result.current.vehicles).toHaveLength(1);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("updateNotificationPrefs sends PATCH with notifyChargeStarted", async () => {
    mockFetch.mockImplementation((url: string, opts?: any) => {
      if (opts?.method === "PATCH" && url.includes("/api/vehicles/")) {
        return Promise.resolve({ ok: true, status: 204 });
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() => useVehicle());

    await waitFor(
      () => {
        expect(result.current.vehicles).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    let ok = false;
    await act(async () => {
      ok = await result.current.updateNotificationPrefs("rm1", {
        notifyChargeStarted: false,
      });
    });

    expect(ok).toBe(true);
    const patchCalls = mockFetch.mock.calls.filter(
      (call) => call[1] && call[1].method === "PATCH",
    );
    expect(patchCalls.length).toBeGreaterThan(0);
    const lastPatch = patchCalls.at(-1);
    if (!lastPatch) throw new Error("no patch calls");
    expect(lastPatch[1].body).toContain('"notifyChargeStarted":false');
  });

  it("fetches when no SSR data provided", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => [vehicles[0]] });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() => useVehicle());

    expect(result.current.isLoading).toBe(true);

    await waitFor(
      () => {
        expect(result.current.vehicles).toHaveLength(1);
      },
      { timeout: 1000 },
    );
  });
});
