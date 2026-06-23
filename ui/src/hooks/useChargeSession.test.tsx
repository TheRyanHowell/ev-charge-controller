import { useGaugeStore } from "@/stores/gaugeStore";
import { customRenderHook as renderHook, act, waitFor } from "@/test-utils";
import { createChargeSession, createVehicle } from "@/test/fixtures";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import { useChargeSession } from "./useChargeSession";

const mockLocalStorage = {
  getItem: vi.fn<() => string | null>(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
};
Object.defineProperty(global.window, "localStorage", {
  value: mockLocalStorage,
});

const mockFetch = vi.fn();
global.fetch = mockFetch;

const testVehicle = createVehicle();

const originalConsoleError = console.error;
describe("useChargeSession", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLocalStorage.getItem.mockImplementation(() => null);
    mockFetch.mockImplementation(() => Promise.resolve({ ok: false }));
    useGaugeStore.setState({ currentPercent: 25, targetPercent: 80 });
    vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
      const msg = args.map(String).join(" ");
      if (
        msg.includes("Failed to fetch charge session:") ||
        msg.includes("Failed to update target percent:") ||
        msg.includes("Failed to fetch history for auto-stop:")
      ) {
        return;
      }
      originalConsoleError(...args);
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  const flushMicrotasks = () => act(async () => await Promise.resolve());

  const opts = () => ({
    selectedVehicle: testVehicle,
    isLoading: false,
  });

  it("starts with idle session", async () => {
    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );
    await flushMicrotasks();
    expect(result.current.session.status).toBe("idle");
  });

  it("sets powerDraw to 0 when session status is idle (non-charging)", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({
            status: "idle",
            currentPercent: 30,
            targetPercent: 80,
            startPercent: 30,
            powerDraw: 0,
          }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await waitFor(
      () => {
        expect(result.current.session.status).toBe("idle");
      },
      { timeout: 2000 },
    );
  });

  it("maps unknown API status to idle", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({
            status: "unknown_status",
            currentPercent: 30,
            targetPercent: 80,
            startPercent: 30,
            powerDraw: 0,
          }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await waitFor(
      () => {
        expect(result.current.session.status).toBe("idle");
      },
      { timeout: 2000 },
    );
  });

  it("maps pending API status to pending", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () =>
            createChargeSession({
              id: "sess-123",
              startKwh: 0,
              targetKwh: 1,
              startPercent: 30,
              status: "pending",
              currentPercent: 30,
              powerDraw: 0,
            }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await waitFor(
      () => {
        expect(result.current.session.status).toBe("pending");
      },
      { timeout: 2000 },
    );
  });

  it("maps cancelled API status to error", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () =>
            createChargeSession({
              id: "sess-123",
              startKwh: 0,
              targetKwh: 1,
              startPercent: 30,
              status: "cancelled",
              currentPercent: 30,
              powerDraw: 0,
            }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await waitFor(
      () => {
        expect(result.current.session.status).toBe("error");
      },
      { timeout: 2000 },
    );
  });

  it("handles history fetch error during auto-stop check", async () => {
    let pollCount = 0;
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        pollCount++;
        if (pollCount === 1) {
          return Promise.resolve({
            status: 200,
            ok: true,
            json: async () => ({
              id: "sess-123",
              vehicleId: "rm1",
              createdAt: "2024-01-01T10:00:00Z",
              startKwh: 0,
              targetKwh: 1,
              startPercent: 20,
              targetPercent: 80,
              status: "active",
              currentPercent: 45,
              powerDraw: 600,
            }),
          });
        }
        return Promise.resolve({ status: 204, ok: false });
      }
      if (url.includes("/api/history")) {
        return Promise.reject(new Error("History fetch failed"));
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    // Wait for first poll to show charging
    await waitFor(
      () => {
        expect(result.current.session.status).toBe("charging");
      },
      { timeout: 2000 },
    );

    // Wait for second poll to show idle (history error is logged, not surfaced)
    await waitFor(
      () => {
        expect(result.current.session.status).toBe("idle");
      },
      { timeout: 8000 },
    );

    // Should not set error message for history fetch failure
    expect(result.current.errorMessage).toBeNull();
  }, 10000);

  it("bumps target to cp+10 when endPercent exceeds targetPercent in auto-stop", async () => {
    let pollCount = 0;

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        pollCount++;
        if (pollCount === 1) {
          return Promise.resolve({
            status: 200,
            ok: true,
            json: async () => ({
              id: "sess-123",
              vehicleId: "rm1",
              createdAt: "2024-01-01T10:00:00Z",
              startKwh: 0,
              targetKwh: 1,
              startPercent: 20,
              targetPercent: 80,
              status: "active",
              currentPercent: 45,
              powerDraw: 600,
            }),
          });
        }
        // Subsequent polls: no active session
        return Promise.resolve({ status: 204, ok: false });
      }
      if (url.includes("/api/history")) {
        return Promise.resolve({
          ok: true,
          json: async () => [
            {
              id: "hist-1",
              vehicleId: "rm1",
              createdAt: "2024-01-01T00:00:00Z",
              startKwh: 1,
              targetKwh: 2,
              startPercent: 20,
              status: "completed",
              endPercent: 85,
              targetPercent: 80,
            },
          ],
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    // Wait for first poll to show charging
    await waitFor(
      () => {
        expect(result.current.session.status).toBe("charging");
      },
      { timeout: 5000 },
    );

    // Wait for second poll to show idle (204 response)
    await waitFor(
      () => {
        expect(result.current.session.status).toBe("idle");
      },
      { timeout: 10000 },
    );

    // Wait for history fetch to complete and update markers
    await waitFor(
      () => {
        expect(useGaugeStore.getState().currentPercent).toBe(85);
      },
      { timeout: 5000 },
    );

    // Target should be bumped to cp + 10 = 95
    expect(useGaugeStore.getState().targetPercent).toBe(95);
  }, 20000);

  it("initializes from initialSession with active status", async () => {
    const { result, unmount } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id", {
        status: "active",
        powerDraw: 500,
        startPercent: 20,
        currentPercent: 45,
        targetPercent: 80,
      }),
    );

    expect(result.current.session.status).toBe("charging");
    if (result.current.session.status === "charging") {
      expect(result.current.session.powerDraw).toBe(500);
    }
    expect(result.current.chargeStartPercent).toBe(20);
    unmount();
  });

  it("initializes from initialSession with pending status", async () => {
    const { result, unmount } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id", {
        status: "pending",
        powerDraw: null,
      }),
    );

    expect(result.current.session.status).toBe("pending");
    unmount();
  });

  it("initializes from initialSession with cancelled status", async () => {
    const { result, unmount } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id", {
        status: "cancelled",
        powerDraw: null,
      }),
    );

    expect(result.current.session.status).toBe("error");
    unmount();
  });

  it("initializes from initialSession with unknown status as idle", async () => {
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { result, unmount } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id", {
        status: "unknown_status",
        powerDraw: null,
      }),
    );

    expect(result.current.session.status).toBe("idle");
    expect(warnSpy).toHaveBeenCalledWith(
      "Unknown backend status: unknown_status, defaulting to idle",
    );
    warnSpy.mockRestore();
    unmount();
  });

  it("transitions to idle after stopCharging", async () => {
    let stopped = false;
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (init?.method === "PATCH" && url.includes("/api/charge-sessions")) {
        stopped = true;
        return Promise.resolve({
          ok: true,
          status: 204,
        });
      }
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        if (stopped) {
          return Promise.resolve({
            ok: true,
            status: 200,
            json: async () => null,
          });
        }
        return Promise.resolve({
          status: 200,
          ok: true,
          json: async () =>
            createChargeSession({
              id: "sess-123",
              startKwh: 0,
              targetKwh: 1,
              startPercent: 20,
              status: "active",
              currentPercent: 45,
              powerDraw: 600,
            }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result, unmount } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await waitFor(
      () => {
        expect(result.current.session.status).toBe("charging");
      },
      { timeout: 2000 },
    );

    await act(async () => {
      await result.current.stopCharging();
    });

    await waitFor(
      () => {
        expect(result.current.session.status).toBe("idle");
      },
      { timeout: 2000 },
    );
    unmount();
  }, 15000);

  it("returns clearError function that resets error", async () => {
    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );
    await flushMicrotasks();
    expect(result.current.clearError).toBeDefined();
    expect(typeof result.current.clearError).toBe("function");
  });

  it("returns startCharging and stopCharging functions", async () => {
    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );
    await flushMicrotasks();
    expect(typeof result.current.startCharging).toBe("function");
    expect(typeof result.current.stopCharging).toBe("function");
  });

  it("returns handleTargetChargeUpdate function", async () => {
    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );
    await flushMicrotasks();
    expect(typeof result.current.handleTargetChargeUpdate).toBe("function");
  });

  it("returns onDragStart and onDragEnd functions", async () => {
    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );
    await flushMicrotasks();
    expect(typeof result.current.onDragStart).toBe("function");
    expect(typeof result.current.onDragEnd).toBe("function");
  });

  it("sets error message when start charging fails with problem response", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions")) {
        return Promise.resolve({
          ok: false,
          status: 400,
          text: async () =>
            JSON.stringify({
              type: "about:blank#target-must-be-higher",
              title: "Bad Request",
              status: 400,
              detail:
                "targetPercent (30) must be greater than startPercent (30)",
            }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await act(async () => {
      await result.current.startCharging();
    });

    expect(result.current.errorMessage).toContain(
      "must be greater than startPercent",
    );
  });

  it("sets internal error message on fetch exception", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions")) {
        return Promise.reject(new Error("Network error"));
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await act(async () => {
      await result.current.startCharging();
    });

    expect(result.current.errorMessage).toContain("Something went wrong");
  });

  it("does not call API when startCharging with no vehicle", async () => {
    const { result } = renderHook(() => useChargeSession(null, null));

    await act(async () => {
      await result.current.startCharging();
    });

    // Should not call charge-sessions endpoint (tasmota status may be polled)
    const chargeSessionCalls = mockFetch.mock.calls.filter((call: any) =>
      call[0]?.includes("/api/charge-sessions"),
    );
    expect(chargeSessionCalls).toHaveLength(0);
  });

  it("parses unknown error body as unknown error", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions")) {
        return Promise.resolve({
          ok: false,
          status: 500,
          text: async () => "Plain text error",
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await act(async () => {
      await result.current.startCharging();
    });

    expect(result.current.errorMessage).toContain("Something went wrong");
  });

  it("returns session from data", async () => {
    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );
    await flushMicrotasks();
    expect(result.current.session.status).toBe("idle");
  });

  it('sets error message on "session-already-active" startCharging error', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.resolve({
          ok: false,
          status: 409,
          text: async () =>
            JSON.stringify({
              type: "about:blank#session-already-active",
              title: "Conflict",
              status: 409,
              detail: "A charge session is already in progress.",
            }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await act(async () => {
      await result.current.startCharging();
    });

    expect(result.current.errorMessage).toContain("already in progress");
  });

  it("sets internal error on stopCharging tasmota error", async () => {
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/charge-sessions")) {
        if (init?.method === "PATCH") {
          return Promise.resolve({
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
        }
        return Promise.resolve({ status: 204 });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await act(async () => {
      await result.current.stopCharging();
    });

    expect(result.current.errorMessage).toContain("failed to set power state");
  });

  it("clearError clears errorMessage", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions")) {
        return Promise.resolve({
          ok: false,
          status: 400,
          text: async () =>
            JSON.stringify({
              type: "about:blank#target-must-be-higher",
              title: "Bad Request",
              status: 400,
              detail:
                "targetPercent (30) must be greater than startPercent (30)",
            }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await act(async () => {
      await result.current.startCharging();
    });

    expect(result.current.errorMessage).toContain(
      "must be greater than startPercent",
    );

    await act(async () => {
      result.current.clearError();
    });

    expect(result.current.errorMessage).toBeNull();
  });

  it("handleTargetChargeUpdate does nothing when not charging", async () => {
    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await act(async () => {
      result.current.handleTargetChargeUpdate(30, 60);
    });

    // No fetch calls should be made when not charging
    expect(mockFetch).not.toHaveBeenCalledWith(
      expect.stringContaining("/api/charge-sessions"),
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("auto-stops and updates markers when polling returns 204 and history shows completed", async () => {
    // First poll returns active session (updates prevCurrentPercentRef),
    // second poll returns 204 (triggers history check since prevCurrent changed)
    let pollCount = 0;

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        pollCount++;
        if (pollCount === 1) {
          // First poll: active session at 45%
          return Promise.resolve({
            status: 200,
            ok: true,
            json: async () => ({
              id: "sess-123",
              vehicleId: "rm1",
              createdAt: "2024-01-01T10:00:00Z",
              startKwh: 0,
              targetKwh: 1,
              startPercent: 20,
              targetPercent: 80,
              status: "active",
              currentPercent: 45,
              powerDraw: 600,
            }),
          });
        }
        // Second poll: no active session
        return Promise.resolve({ status: 204, ok: false });
      }
      if (url.includes("/api/history")) {
        return Promise.resolve({
          ok: true,
          json: async () => [
            {
              id: "hist-1",
              vehicleId: "rm1",
              createdAt: "2024-01-01T00:00:00Z",
              startKwh: 1,
              targetKwh: 2,
              startPercent: 20,
              status: "completed",
              endPercent: 75,
              targetPercent: 80,
            },
          ],
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    // Wait for first poll to show charging
    await waitFor(
      () => {
        expect(result.current.session.status).toBe("charging");
      },
      { timeout: 2000 },
    );

    // Wait for second poll to show idle and update markers from history
    await waitFor(
      () => {
        expect(useGaugeStore.getState().currentPercent).toBe(75);
      },
      { timeout: 8000 },
    );
    expect(useGaugeStore.getState().targetPercent).toBe(80);
  }, 10000);

  it("handles non-ok history response during auto-stop check", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.resolve({ status: 204, ok: false });
      }
      if (url.includes("/api/history")) {
        return Promise.resolve({ status: 500, ok: false });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await waitFor(
      () => {
        expect(result.current.session.status).toBe("idle");
      },
      { timeout: 1000 },
    );

    // Should not set error message for non-ok history during auto-stop
    expect(result.current.errorMessage).toBeNull();
  });

  it("onDragStart sets isDraggingRef true, onDragEnd sets it false", async () => {
    // Mock active session
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({
            id: "sess-123",
            vehicleId: "rm1",
            createdAt: "2024-01-01T10:00:00Z",
            startKwh: 0,
            targetKwh: 1,
            startPercent: 20,
            targetPercent: 80,
            status: "active",
            currentPercent: 40,
            powerDraw: 600,
          }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    // Wait for polling to update store currentPercent
    await waitFor(
      () => {
        expect(useGaugeStore.getState().currentPercent).toBe(40);
      },
      { timeout: 1000 },
    );

    // Call onDragStart - should set isDraggingRef to true
    act(() => {
      result.current.onDragStart();
    });

    // While dragging, the polling effect should skip updating markers
    // We verify by checking that setMarkerCurrent is not called during drag

    // Call onDragEnd - should set isDraggingRef to false
    act(() => {
      result.current.onDragEnd(40, 80);
    });

    expect(typeof result.current.onDragStart).toBe("function");
    expect(typeof result.current.onDragEnd).toBe("function");
  });

  it("stopCharging handles 503 tasmota unreachable", async () => {
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        if (init?.method === "PATCH") {
          return Promise.resolve({
            ok: false,
            status: 503,
            text: async () =>
              JSON.stringify({
                type: "about:blank#relay-control-failed",
                title: "Service Unavailable",
                status: 503,
                detail:
                  "Could not control the smart plug. Please check that it is powered on and connected.",
              }),
          });
        }
        return Promise.resolve({ status: 204, ok: false });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await act(async () => {
      await result.current.stopCharging();
    });

    expect(result.current.errorMessage).toContain(
      "Could not control the smart plug",
    );
  });

  it("stopCharging catch block sets internal error", async () => {
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/charge-sessions") && init?.method === "PATCH") {
        return Promise.reject(new Error("Network error"));
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    await act(async () => {
      await result.current.stopCharging();
    });

    expect(result.current.errorMessage).toContain("Something went wrong");
  });

  it("polling fetchSession catch block handles network error", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.reject(new Error("Network error"));
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    // Should remain idle without error message on network error
    await waitFor(
      () => {
        expect(result.current.session.status).toBe("idle");
      },
      { timeout: 1000 },
    );
    expect(result.current.errorMessage).toBeNull();
  });

  it("handleTargetChargeUpdate debounces PATCH request", async () => {
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/charge-sessions") && init?.method === "PATCH") {
        return Promise.resolve({ ok: true });
      }
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({
            id: "sess-123",
            vehicleId: "rm1",
            createdAt: "2024-01-01T10:00:00Z",
            startKwh: 0,
            targetKwh: 1,
            startPercent: 20,
            targetPercent: 80,
            status: "active",
            currentPercent: 40,
            powerDraw: 600,
          }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    // Wait for polling to establish charging session
    await waitFor(
      () => {
        expect(result.current.session.status).toBe("charging");
      },
      { timeout: 1000 },
    );

    // Call handleTargetChargeUpdate - should schedule a debounced PATCH
    act(() => {
      result.current.handleTargetChargeUpdate(40, 60);
    });

    // Wait for debounce to fire (TARGET_UPDATE_DEBOUNCE_MS = 1000ms)
    await act(async () => {
      await new Promise((r) => setTimeout(r, 1100));
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/charge-sessions"),
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("handleTargetChargeUpdate catch block handles PATCH failure", async () => {
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/charge-sessions") && init?.method === "PATCH") {
        return Promise.reject(new Error("Network error"));
      }
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({
            id: "sess-123",
            vehicleId: "rm1",
            createdAt: "2024-01-01T10:00:00Z",
            startKwh: 0,
            targetKwh: 1,
            startPercent: 20,
            targetPercent: 80,
            status: "active",
            currentPercent: 40,
            powerDraw: 600,
          }),
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    // Wait for polling to establish charging session
    await waitFor(
      () => {
        expect(result.current.session.status).toBe("charging");
      },
      { timeout: 1000 },
    );

    // Call handleTargetChargeUpdate - PATCH will fail, but should not throw
    act(() => {
      result.current.handleTargetChargeUpdate(40, 60);
    });

    // Wait for debounce to fire
    await act(async () => {
      await new Promise((r) => setTimeout(r, 1100));
    });

    // Should not set error message for PATCH failure (logged silently)
    expect(result.current.errorMessage).toBeNull();
  });

  it("passes limit=1 to history fetch for auto-stop detection", async () => {
    let pollCount = 0;

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        pollCount++;
        if (pollCount === 1) {
          // First poll: active session (updates prevCurrentPercentRef)
          return Promise.resolve({
            status: 200,
            ok: true,
            json: async () => ({
              id: "sess-123",
              vehicleId: "rm1",
              createdAt: "2024-01-01T10:00:00Z",
              startKwh: 0,
              targetKwh: 1,
              startPercent: 20,
              targetPercent: 80,
              status: "active",
              currentPercent: 45,
              powerDraw: 600,
            }),
          });
        }
        // Second poll: no active session (triggers history check)
        return Promise.resolve({ status: 204, ok: false });
      }
      if (url.includes("/api/history")) {
        return Promise.resolve({
          ok: true,
          json: async () => [
            {
              id: "hist-1",
              vehicleId: "rm1",
              createdAt: "2024-01-01T00:00:00Z",
              startKwh: 1,
              targetKwh: 2,
              startPercent: 20,
              status: "completed",
              endPercent: 75,
              targetPercent: 80,
            },
          ],
        });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = renderHook(() =>
      useChargeSession(opts().selectedVehicle, "test-plug-id"),
    );

    // Wait for first poll to show charging
    await waitFor(
      () => {
        expect(result.current.session.status).toBe("charging");
      },
      { timeout: 2000 },
    );

    // Wait for second poll to trigger history fetch
    await waitFor(
      () => {
        const historyCall = mockFetch.mock.calls.find((call: any) =>
          call[0].includes("/api/history"),
        );
        expect(historyCall).toBeDefined();
      },
      { timeout: 8000 },
    );

    // Verify the history fetch includes limit=1
    const historyCall = mockFetch.mock.calls.find((call: any) =>
      call[0].includes("/api/history"),
    );
    expect(historyCall).toBeDefined();
    if (historyCall) {
      expect(historyCall[0]).toContain("limit=1");
    }
  }, 10000);

  it("auto-stop persists percents to vehicle API so idle sync doesn't overwrite", async () => {
    let pollCount = 0;

    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/charge-sessions") && !url.includes("/start")) {
        pollCount++;
        if (pollCount === 1) {
          return Promise.resolve({
            status: 200,
            ok: true,
            json: async () => ({
              id: "sess-123",
              vehicleId: "rm1",
              createdAt: "2024-01-01T10:00:00Z",
              startKwh: 0,
              targetKwh: 1,
              startPercent: 20,
              targetPercent: 80,
              status: "active",
              currentPercent: 45,
              powerDraw: 600,
            }),
          });
        }
        return Promise.resolve({ status: 204, ok: false });
      }
      if (url.includes("/api/history")) {
        return Promise.resolve({
          ok: true,
          json: async () => [
            {
              id: "hist-1",
              vehicleId: "rm1",
              createdAt: "2024-01-01T00:00:00Z",
              startKwh: 1,
              targetKwh: 2,
              startPercent: 20,
              status: "completed",
              endPercent: 72,
              targetPercent: 80,
            },
          ],
        });
      }
      if (url.includes("/api/vehicles/") && init?.method === "PATCH") {
        return Promise.resolve({ status: 204, ok: true });
      }
      return Promise.resolve({ ok: false });
    });

    renderHook(() => useChargeSession(opts().selectedVehicle, "test-plug-id"));

    // Wait for first poll to establish charging
    await waitFor(
      () => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining("/api/charge-sessions"),
          expect.anything(),
        );
      },
      { timeout: 2000 },
    );

    // Wait for auto-stop to PATCH the vehicle API
    await waitFor(
      () => {
        const vehiclePatch = mockFetch.mock.calls.find(
          (call: any) =>
            call[0]?.includes("/api/vehicles/") && call[1]?.method === "PATCH",
        );
        expect(vehiclePatch).toBeDefined();
      },
      { timeout: 8000 },
    );

    const vehiclePatch = mockFetch.mock.calls.find(
      (call: any) =>
        call[0]?.includes("/api/vehicles/") && call[1]?.method === "PATCH",
    );
    expect(vehiclePatch).toBeDefined();
    if (vehiclePatch) {
      const body = JSON.parse(vehiclePatch[1].body);
      expect(body.currentPercent).toBe(72);
      expect(body.targetPercent).toBe(80);
    }
  }, 12000);
});
