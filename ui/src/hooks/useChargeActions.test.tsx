import { useErrorHandling } from "@/hooks/useErrorHandling";
import { queryKeys } from "@/lib/queryKeys";
import { createChargeSession, createVehicle } from "@/test/fixtures";
import {
  QueryClient,
  QueryClientProvider,
  useQueryClient,
} from "@tanstack/react-query";
import { renderHook, act } from "@testing-library/react";
import { useMemo } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import { useChargeActions, type ChargeActionsDeps } from "./useChargeActions";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const testVehicle = createVehicle();

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0 },
      mutations: { retry: false },
    },
  });
}

function TestWrapper({ children }: { children: React.ReactNode }) {
  const queryClient = useMemo(() => makeQueryClient(), []);
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

function renderChargeActions(overrides: Partial<ChargeActionsDeps> = {}) {
  return renderHook(
    () => {
      const { errorMessage, onError, clearError } = useErrorHandling();
      const queryClient = useQueryClient();

      const deps: ChargeActionsDeps = {
        vehicle: testVehicle,
        plugId: "test-plug-id",
        currentPercent: 30,
        targetPercent: 80,
        sessionStatus: "idle",
        onError,
        ...overrides,
      };

      const actions = useChargeActions(deps);

      return {
        ...actions,
        queryClient,
        errorMessage,
        clearError,
      };
    },
    { wrapper: TestWrapper },
  );
}

describe("useChargeActions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("does not call API when startCharging with no vehicle", async () => {
    const { result } = renderChargeActions({ vehicle: null });

    expect(result.current.startCharging.mutateAsync).toBeDefined();
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("sets pending status optimistically on start", async () => {
    const mockSession = createChargeSession({ status: "pending" });
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockSession,
    });

    const { result } = renderChargeActions();

    await act(async () => {
      await result.current.startCharging.mutateAsync({
        vehicleId: "rm1",
        startPercent: 30,
        targetPercent: 80,
      });
    });

    const cacheData = result.current.queryClient.getQueryData(
      queryKeys.chargeSession.byVehicle(testVehicle.id),
    );
    expect(cacheData).toEqual(mockSession);
  });

  it("sets error from backend detail on start failure", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      text: async () =>
        JSON.stringify({
          type: "about:blank#target-must-be-higher",
          title: "Bad Request",
          status: 400,
          detail: "targetPercent (30) must be greater than startPercent (30)",
        }),
    });

    const { result } = renderChargeActions();

    await act(async () => {
      await result.current.startCharging
        .mutateAsync({
          vehicleId: "rm1",
          startPercent: 30,
          targetPercent: 80,
        })
        .catch(() => {});
    });

    expect(result.current.errorMessage).toContain("must be greater than");
  });

  it("sets internal error on fetch exception", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = renderChargeActions();

    await act(async () => {
      await result.current.startCharging
        .mutateAsync({
          vehicleId: "rm1",
          startPercent: 30,
          targetPercent: 80,
        })
        .catch(() => {});
    });

    expect(result.current.errorMessage).toContain("Something went wrong");
  });

  it("resets to idle on start failure", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = renderChargeActions({
      sessionStatus: "charging",
    });

    await act(async () => {
      await result.current.startCharging
        .mutateAsync({
          vehicleId: "rm1",
          startPercent: 30,
          targetPercent: 80,
        })
        .catch(() => {});
    });

    const cacheData = result.current.queryClient.getQueryData(
      queryKeys.chargeSession.byVehicle(testVehicle.id),
    );
    expect(cacheData).toEqual({ status: "idle" });
  });

  it("handles stopCharging success", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const { result } = renderChargeActions();

    await act(async () => {
      await result.current.stopCharging.mutateAsync();
    });

    const cacheData = result.current.queryClient.getQueryData(
      queryKeys.chargeSession.byVehicle(testVehicle.id),
    );
    expect(cacheData).toBeNull();
  });

  it("handles stopCharging 204 (no active session)", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const { result } = renderChargeActions();

    await act(async () => {
      await result.current.stopCharging.mutateAsync();
    });

    const cacheData = result.current.queryClient.getQueryData(
      queryKeys.chargeSession.byVehicle(testVehicle.id),
    );
    expect(cacheData).toBeNull();
  });

  it("sets error from backend detail on stopCharging failure", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 503,
      text: async () =>
        JSON.stringify({
          type: "about:blank#relay-control-failed",
          title: "Service Unavailable",
          status: 503,
          detail:
            'failed to set power state: Post "http://192.168.0.100/cm?cmnd=Power1%20OFF": context deadline exceeded',
        }),
    });

    const { result } = renderChargeActions();

    await act(async () => {
      await result.current.stopCharging.mutateAsync().catch(() => {});
    });

    expect(result.current.errorMessage).toContain("failed to set power state");
  });

  it("does not clear cache on stopCharging failure", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = renderChargeActions({ sessionStatus: "charging" });

    result.current.queryClient.setQueryData(
      queryKeys.chargeSession.byVehicle(testVehicle.id),
      {
        status: "active",
        powerDraw: 600,
      },
    );

    expect(
      result.current.queryClient.getQueryData(
        queryKeys.chargeSession.byVehicle(testVehicle.id),
      ),
    ).toEqual({
      status: "active",
      powerDraw: 600,
    });

    await act(async () => {
      await result.current.stopCharging.mutateAsync().catch(() => {});
    });

    // Cache should not be cleared on failure
    expect(
      result.current.queryClient.getQueryData(
        queryKeys.chargeSession.byVehicle(testVehicle.id),
      ),
    ).toEqual({
      status: "active",
      powerDraw: 600,
    });
  });

  it("handles startCharging with active session response", async () => {
    const mockSession = createChargeSession({
      status: "active",
      powerDraw: 600,
    });
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockSession,
    });

    const { result } = renderChargeActions();

    await act(async () => {
      await result.current.startCharging.mutateAsync({
        vehicleId: "rm1",
        startPercent: 30,
        targetPercent: 80,
      });
    });

    const cacheData = result.current.queryClient.getQueryData(
      queryKeys.chargeSession.byVehicle(testVehicle.id),
    );
    expect(cacheData).toEqual(mockSession);
  });

  it("handleTargetChargeUpdate does nothing when not charging", async () => {
    const { result } = renderChargeActions();

    await act(async () => {
      result.current.handleTargetChargeUpdate(30, 60);
      await new Promise((r) => setTimeout(r, 350));
    });

    expect(mockFetch).not.toHaveBeenCalledWith(
      expect.stringContaining("/api/charge-sessions"),
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("handleTargetChargeUpdate debounces PATCH when charging", async () => {
    mockFetch.mockImplementation((_url: string, init?: RequestInit) => {
      if (init?.method === "PATCH") {
        return Promise.resolve({ ok: true });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderChargeActions({ sessionStatus: "charging" });

    act(() => {
      result.current.handleTargetChargeUpdate(40, 60);
    });

    expect(mockFetch).not.toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ method: "PATCH" }),
    );

    await act(async () => {
      await new Promise((r) => setTimeout(r, 350));
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("handleTargetChargeUpdate calls onTargetUpdateError on failure", async () => {
    const onTargetUpdateError = vi.fn();
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = renderChargeActions({
      sessionStatus: "charging",
      onTargetUpdateError,
    });

    act(() => {
      result.current.handleTargetChargeUpdate(40, 60);
    });

    await act(async () => {
      await new Promise((r) => setTimeout(r, 350));
    });

    expect(onTargetUpdateError).toHaveBeenCalledWith(
      expect.stringContaining("Something went wrong"),
    );
  });

  it("handleTargetChargeUpdate optimistically updates the session cache", async () => {
    mockFetch.mockImplementation((_url: string, init?: RequestInit) =>
      init?.method === "PATCH"
        ? Promise.resolve({ ok: true, status: 204 })
        : Promise.resolve({ ok: false }),
    );

    const { result } = renderChargeActions({ sessionStatus: "charging" });
    act(() => {
      result.current.queryClient.setQueryData(
        queryKeys.chargeSession.byVehicle(testVehicle.id),
        { status: "active", targetPercent: 80, powerDraw: 600 },
      );
    });

    act(() => {
      result.current.handleTargetChargeUpdate(40, 95);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 350));
    });

    const cacheData = result.current.queryClient.getQueryData(
      queryKeys.chargeSession.byVehicle(testVehicle.id),
    ) as { targetPercent: number };
    expect(cacheData.targetPercent).toBe(95);
  });

  it("handleTargetChargeUpdate rolls back the cache on failure", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 503,
      text: async () => "",
    });

    const { result } = renderChargeActions({ sessionStatus: "charging" });
    act(() => {
      result.current.queryClient.setQueryData(
        queryKeys.chargeSession.byVehicle(testVehicle.id),
        { status: "active", targetPercent: 80, powerDraw: 600 },
      );
    });

    act(() => {
      result.current.handleTargetChargeUpdate(40, 95);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 350));
    });

    const cacheData = result.current.queryClient.getQueryData(
      queryKeys.chargeSession.byVehicle(testVehicle.id),
    ) as { targetPercent: number };
    expect(cacheData.targetPercent).toBe(80);
  });

  it("handleTargetChargeUpdate invalidates the plug's schedule cache on settle", async () => {
    mockFetch.mockImplementation((_url: string, init?: RequestInit) =>
      init?.method === "PATCH"
        ? Promise.resolve({ ok: true, status: 204 })
        : Promise.resolve({ ok: false }),
    );

    const { result } = renderChargeActions({ sessionStatus: "charging" });
    const scheduleKey = queryKeys.plugs.schedule("test-plug-id");
    act(() => {
      result.current.queryClient.setQueryData(scheduleKey, {
        id: "sched1",
        type: "carbon_aware",
        time: "01:00",
        windowStart: "01:00",
        windowEnd: "06:00",
        estimatedStartTime: "03:00",
        enabled: true,
      });
    });

    act(() => {
      result.current.handleTargetChargeUpdate(40, 95);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 350));
    });

    expect(
      result.current.queryClient.getQueryState(scheduleKey)?.isInvalidated,
    ).toBe(true);
  });

  it("handleTargetChargeUpdate invalidates the plug's schedule cache even when the PATCH fails", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 503,
      text: async () => "",
    });

    const { result } = renderChargeActions({ sessionStatus: "charging" });
    const scheduleKey = queryKeys.plugs.schedule("test-plug-id");
    act(() => {
      result.current.queryClient.setQueryData(scheduleKey, {
        id: "sched1",
        type: "carbon_aware",
        time: "01:00",
        enabled: true,
      });
    });

    act(() => {
      result.current.handleTargetChargeUpdate(40, 95);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 350));
    });

    expect(
      result.current.queryClient.getQueryState(scheduleKey)?.isInvalidated,
    ).toBe(true);
  });

  it("handleTargetChargeUpdate does not touch the schedule cache when plugId is null", async () => {
    mockFetch.mockImplementation((_url: string, init?: RequestInit) =>
      init?.method === "PATCH"
        ? Promise.resolve({ ok: true, status: 204 })
        : Promise.resolve({ ok: false }),
    );

    const { result } = renderChargeActions({
      sessionStatus: "charging",
      plugId: null,
    });

    act(() => {
      result.current.handleTargetChargeUpdate(40, 95);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 350));
    });

    // No plugId means no schedule query exists for this session - nothing to invalidate,
    // and no crash from trying to build a query key with a null plugId.
    expect(
      result.current.queryClient.getQueryState(queryKeys.plugs.schedule("")),
    ).toBeUndefined();
  });

  it("handleTargetChargeUpdate holds the commit guard then releases it on settle", async () => {
    const setCommitting = vi.fn();
    mockFetch.mockImplementation((_url: string, init?: RequestInit) =>
      init?.method === "PATCH"
        ? Promise.resolve({ ok: true, status: 204 })
        : Promise.resolve({ ok: false }),
    );

    const { result } = renderChargeActions({
      sessionStatus: "charging",
      setCommitting,
    });

    act(() => {
      result.current.handleTargetChargeUpdate(40, 95);
    });
    // Held immediately (covers the debounce window before the request fires).
    expect(setCommitting).toHaveBeenCalledWith(true);

    await act(async () => {
      await new Promise((r) => setTimeout(r, 350));
    });
    // Released once the write settles.
    expect(setCommitting).toHaveBeenLastCalledWith(false);
  });

  it("handleTargetChargeUpdate handles backend error detail", async () => {
    const onTargetUpdateError = vi.fn();
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      text: async () =>
        JSON.stringify({
          type: "about:blank#invalid-target",
          title: "Bad Request",
          status: 400,
          detail: "target percent must be between 1 and 100",
        }),
    });

    const { result } = renderChargeActions({
      sessionStatus: "charging",
      onTargetUpdateError,
    });

    act(() => {
      result.current.handleTargetChargeUpdate(40, 150);
    });

    await act(async () => {
      await new Promise((r) => setTimeout(r, 350));
    });

    expect(onTargetUpdateError).toHaveBeenCalledWith(
      expect.stringContaining("must be between"),
    );
  });
});
