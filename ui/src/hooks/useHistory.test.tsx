import { HistoryChargeSession, HistoryVehicle } from "@/lib/schemas";
import { customRenderHook, act, waitFor } from "@/test-utils";
import { createHistorySession, createHistoryVehicle } from "@/test/fixtures";
import {
  formatDuration,
  formatTimeRange,
  getStatusBadgeClass,
  getStatusColor,
  getTotalEnergy,
} from "@/utils/history";
import { describe, it, expect, vi, beforeEach } from "vitest";

import { useHistory } from "./useHistory";

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

const mockVehicles: HistoryVehicle[] = [
  createHistoryVehicle({ id: "v1", capacityKwh: 3.8, chargerOutputW: 1200 }),
];

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
];

const mockSession = mockSessions[0] as HistoryChargeSession;

function cloneSession(
  overrides: Partial<HistoryChargeSession> = {},
): HistoryChargeSession {
  return { ...mockSession, ...overrides };
}

describe("useHistory", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLocalStorage.getItem.mockImplementation(() => null);
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => [] });
      }
      return Promise.resolve({ status: 204, ok: false });
    });
  });

  const setupFetch = (vehicles = mockVehicles, sessions = mockSessions) => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => vehicles });
      }
      if (url.includes("/api/history")) {
        if (sessions.length === 0) {
          return Promise.resolve({ status: 204, ok: false });
        }
        const params = new URLSearchParams(url.split("?")[1] || "");
        const vehicleId = params.get("vehicleId");
        const date = params.get("date");
        // Update session dates to match the requested date so tests work with any date
        const updatedSessions = sessions.map((s) => {
          if (date) {
            const newStart = `${date}T10:00:00Z`;
            const newEnd = `${date}T12:30:00Z`;
            return {
              ...s,
              createdAt: newStart,
              endedAt: newEnd,
            };
          }
          return s;
        });
        const filtered = vehicleId
          ? updatedSessions.filter((s) => s.vehicleId === vehicleId)
          : updatedSessions;
        return Promise.resolve({ ok: true, json: async () => filtered });
      }
      return Promise.resolve({ ok: false });
    });
  };

  it("starts with loading true while fetching", () => {
    mockFetch.mockImplementation(() => new Promise(() => {}));
    const { result } = customRenderHook(() => useHistory());
    expect(result.current.loading).toBe(true);
  });

  it("loads vehicles and sessions on mount", async () => {
    setupFetch();
    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    expect(result.current.sessions).toHaveLength(1);
    expect(result.current.vehicles).toHaveLength(1);
    expect(result.current.error).toBeNull();
  });

  it("filters sessions by selected vehicle", async () => {
    const vm = [
      {
        id: "v1",
        name: "Car A",
        capacityKwh: 3.8,
        chargerOutputW: 1200,
        chargingEfficiency: 0.8,
        rangeMinMi: 100,
        rangeMaxMi: 150,
      },
    ];
    const sm = [
      cloneSession({ vehicleId: "v1" }),
      cloneSession({ id: "s2", vehicleId: "v2" }),
    ];
    setupFetch(vm, sm);

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    await act(async () => {
      result.current.handleVehicleChange("v1");
    });

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );
    expect(result.current.sessions[0]?.vehicleId).toBe("v1");
  });

  it("resets expansion when vehicle filter changes", async () => {
    setupFetch();
    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    await act(async () => {
      result.current.toggleExpand("s1");
    });
    expect(result.current.isExpanded("s1")).toBe(true);

    await act(async () => {
      result.current.handleVehicleChange("all");
    });
    expect(result.current.isExpanded("s1")).toBe(false);
  });

  it("toggles session expansion", async () => {
    setupFetch();
    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    expect(result.current.isExpanded("s1")).toBe(false);

    await act(async () => {
      result.current.toggleExpand("s1");
    });
    expect(result.current.isExpanded("s1")).toBe(true);

    await act(async () => {
      result.current.toggleExpand("s1");
    });
    expect(result.current.isExpanded("s1")).toBe(false);
  });

  it("formats duration correctly", async () => {
    expect(formatDuration("2024-01-15T10:00:00Z", "2024-01-15T12:30:00Z")).toBe(
      "2h 30m",
    );
    expect(formatDuration("2024-01-15T10:00:00Z")).toBe("In progress");
    expect(formatDuration("2024-01-15T10:00:00Z", "2024-01-15T10:25:00Z")).toBe(
      "25 min",
    );
  });

  it("formats time range in 24-hour format", async () => {
    const range = formatTimeRange(
      "2024-01-15T10:00:00Z",
      "2024-01-15T12:30:00Z",
    );
    expect(range).toMatch(/\d{2}:\d{2}/);
    expect(range).toContain("10:00");
    expect(range).toContain("12:30");
  });

  it("formats time range with trailing dash for active sessions", async () => {
    const range = formatTimeRange("2024-01-15T10:00:00Z");
    expect(range).toContain("10:00");
    expect(range).toContain("–");
    expect(range).not.toContain("ongoing");
  });

  it("formats time range without 12-hour clock", async () => {
    const range = formatTimeRange(
      "2024-01-15T22:00:00Z",
      "2024-01-16T00:30:00Z",
    );
    expect(range).toContain("22:00");
    expect(range).toContain("00:30");
    expect(range).not.toMatch(/am|pm/i);
  });

  it("returns total energy from totalBatteryKwh", async () => {
    expect(
      getTotalEnergy({
        totalBatteryKwh: 2.85,
      } as HistoryChargeSession),
    ).toBe("2.85");
    expect(getTotalEnergy({} as HistoryChargeSession)).toBe("-");
  });

  it("returns correct status colors", async () => {
    expect(getStatusColor("completed")).toBe("bg-emerald-500");
    expect(getStatusColor("cancelled")).toBe("bg-red-500");
    expect(getStatusColor("active")).toBe("bg-blue-500");
    expect(getStatusColor("unknown")).toBe("bg-gray-500");
  });

  it("returns correct status badge classes", async () => {
    expect(getStatusBadgeClass("completed")).toContain("emerald");
    expect(getStatusBadgeClass("cancelled")).toContain("red");
    expect(getStatusBadgeClass("active")).toContain("blue");
  });

  it("returns vehicle name or falls back to id", async () => {
    setupFetch();
    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.vehicles).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    expect(result.current.getVehicleName("v1")).toBe("Maeving RM1");
    expect(result.current.getVehicleName("unknown")).toBe("unknown");
  });

  it("defaults to all vehicles on mount (ignores localStorage)", async () => {
    mockLocalStorage.getItem.mockReturnValue("v1");
    setupFetch();

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.selectedVehicleId).toBeNull();
      },
      { timeout: 1000 },
    );

    expect(result.current.selectedVehicleId).toBeNull();
  });

  it("handles API connection failure", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.error).toBe(
          "Unable to connect to the API server. Is the backend running?",
        );
      },
      { timeout: 1000 },
    );
  });

  it("returns default gray class for unknown status", async () => {
    expect(getStatusBadgeClass("unknown")).toBe(
      "bg-gray-500/20 text-gray-400 border-gray-500/30",
    );
    expect(getStatusColor("unknown")).toBe("bg-gray-500");
  });

  it('handles vehicle change to "all"', async () => {
    setupFetch();
    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    await act(async () => {
      result.current.handleVehicleChange("all");
    });

    expect(result.current.selectedVehicleId).toBeNull();
  });

  it("sets error message on non-200 response", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => mockVehicles });
      }
      if (url.includes("/api/history")) {
        return Promise.resolve({ status: 500, ok: false });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.error).not.toBeNull();
      },
      { timeout: 1000 },
    );

    expect(result.current.error).toBe(
      "Unable to connect to the API server. Is the backend running?",
    );
  });

  it("passes date parameter to history API on initial fetch", async () => {
    setupFetch();

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    const historyCalls = mockFetch.mock.calls.filter((call) =>
      call[0].includes("/api/history"),
    );
    expect(historyCalls.length).toBeGreaterThan(0);
    for (const call of historyCalls) {
      expect(call[0]).toContain("date=");
    }
    // Verify selectedDate defaults to today
    const today = new Date().toISOString().split("T")[0];
    expect(result.current.selectedDate).toBe(today);
  });

  it("refetches when selectedDate changes", async () => {
    setupFetch();

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    const initialCalls = mockFetch.mock.calls.filter((call) =>
      call[0].includes("/api/history"),
    ).length;

    await act(async () => {
      result.current.setSelectedDate("2025-01-15");
    });

    await waitFor(
      () => {
        expect(
          mockFetch.mock.calls.filter((call) =>
            call[0].includes("/api/history"),
          ).length,
        ).toBeGreaterThan(initialCalls);
      },
      { timeout: 1000 },
    );

    expect(result.current.selectedDate).toBe("2025-01-15");
  });

  it("refetches with date when vehicle filter changes", async () => {
    setupFetch();

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    await act(async () => {
      result.current.handleVehicleChange("v1");
    });

    const historyCalls = mockFetch.mock.calls.filter((call) =>
      call[0].includes("/api/history"),
    );
    // The last call should include both vehicleId and date
    const lastCall = historyCalls.at(-1);
    if (!lastCall) throw new Error("no history calls");
    expect(lastCall[0]).toContain("vehicleId=v1");
    expect(lastCall[0]).toContain("date=");
  });

  it("handles empty date results", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => mockVehicles });
      }
      if (url.includes("/api/history")) {
        return Promise.resolve({ status: 204, ok: false });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(0);
      },
      { timeout: 1000 },
    );

    expect(result.current.error).toBeNull();
  });

  it("sends limit and offset params to history API", async () => {
    setupFetch();

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    const historyCalls = mockFetch.mock.calls.filter((call) =>
      call[0].includes("/api/history"),
    );
    expect(historyCalls.length).toBeGreaterThan(0);
    for (const call of historyCalls) {
      expect(call[0]).toContain("limit=50");
      expect(call[0]).toContain("offset=0");
    }
  });

  it("loadMore increments offset and appends sessions", async () => {
    // Generate 50 sessions for initial fetch (keeps hasMore=true)
    const page1 = Array.from({ length: 50 }, (_, i) => ({
      ...mockSessions[0],
      id: `p1-s${i}`,
    }));

    // Custom mock: 50 sessions for initial fetch, 1 for page 2
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => mockVehicles });
      }
      if (url.includes("/api/history")) {
        const params = new URLSearchParams(url.split("?")[1] || "");
        const offset = parseInt(params.get("offset") || "0");
        if (offset > 0) {
          return Promise.resolve({
            ok: true,
            json: async () => [
              { ...mockSessions[0], id: "page2-s1", vehicleId: "v1" },
            ],
          });
        }
        return Promise.resolve({ ok: true, json: async () => page1 });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(50);
      },
      { timeout: 1000 },
    );

    const initialCount = result.current.sessions.length;

    await act(async () => {
      result.current.loadMore();
    });

    await waitFor(
      () => {
        expect(result.current.sessions.length).toBeGreaterThan(initialCount);
      },
      { timeout: 1000 },
    );

    // Should have appended the new session
    expect(result.current.sessions).toHaveLength(51);
  });

  it("resets offset when vehicle changes", async () => {
    const page1 = Array.from({ length: 50 }, (_, i) => ({
      ...mockSessions[0],
      id: `p1v-s${i}`,
    }));

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => mockVehicles });
      }
      if (url.includes("/api/history")) {
        const params = new URLSearchParams(url.split("?")[1] || "");
        const offset = parseInt(params.get("offset") || "0");
        if (offset > 0) {
          return Promise.resolve({
            ok: true,
            json: async () => [{ ...mockSessions[0], id: "page2-vehicle" }],
          });
        }
        return Promise.resolve({ ok: true, json: async () => page1 });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(50);
      },
      { timeout: 1000 },
    );

    // First do a loadMore to increment offset
    await act(async () => {
      result.current.loadMore();
    });

    await waitFor(
      () => {
        const calls = mockFetch.mock.calls.filter((c) =>
          c[0].includes("/api/history"),
        );
        // Check that a call with offset > 0 was made
        return calls.some((c) => c[0].includes("offset=50"));
      },
      { timeout: 1000 },
    );

    // Now change vehicle - should reset offset to 0
    await act(async () => {
      result.current.handleVehicleChange("v1");
    });

    const historyCalls = mockFetch.mock.calls.filter((call) =>
      call[0].includes("/api/history"),
    );
    // The last call should have offset=0
    const lastCall = historyCalls.at(-1);
    if (!lastCall) throw new Error("no history calls");
    expect(lastCall[0]).toContain("offset=0");
  });

  it("resets offset when date changes", async () => {
    const page1 = Array.from({ length: 50 }, (_, i) => ({
      ...mockSessions[0],
      id: `p1d-s${i}`,
    }));

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => mockVehicles });
      }
      if (url.includes("/api/history")) {
        const params = new URLSearchParams(url.split("?")[1] || "");
        const offset = parseInt(params.get("offset") || "0");
        if (offset > 0) {
          return Promise.resolve({
            ok: true,
            json: async () => [{ ...mockSessions[0], id: "page2-date" }],
          });
        }
        return Promise.resolve({ ok: true, json: async () => page1 });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(50);
      },
      { timeout: 1000 },
    );

    // First do a loadMore to increment offset
    await act(async () => {
      result.current.loadMore();
    });

    await waitFor(
      () => {
        const calls = mockFetch.mock.calls.filter((c) =>
          c[0].includes("/api/history"),
        );
        // Check that a call with offset > 0 was made
        return calls.some((c) => c[0].includes("offset=50"));
      },
      { timeout: 1000 },
    );

    // Now change date - should reset offset to 0
    await act(async () => {
      result.current.setSelectedDate("2025-06-01");
    });

    const historyCalls = mockFetch.mock.calls.filter((call) =>
      call[0].includes("/api/history"),
    );
    // The last call should have offset=0
    const lastCall = historyCalls.at(-1);
    if (!lastCall) throw new Error("no history calls");
    expect(lastCall[0]).toContain("offset=0");
  });

  it("loadMore does nothing when hasMore is false", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => mockVehicles });
      }
      if (url.includes("/api/history")) {
        return Promise.resolve({ status: 204, ok: false });
      }
      return Promise.resolve({ ok: false });
    });

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.hasMore).toBe(false);
      },
      { timeout: 1000 },
    );

    await act(async () => {
      result.current.loadMore();
    });

    // Should still have no more pages and sessions unchanged
    expect(result.current.hasMore).toBe(false);
    expect(result.current.sessions).toHaveLength(0);
  });

  it("deleteSession removes session from list on success", async () => {
    setupFetch();

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    // Add DELETE handling while preserving existing vehicle/history mocks
    const prevImpl = mockFetch.getMockImplementation();
    let deleteCalled = false;
    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (init?.method === "DELETE") {
        deleteCalled = true;
        return Promise.resolve({ ok: true, status: 200 });
      }
      // After delete, history endpoint returns empty (session was deleted)
      if (deleteCalled && url.includes("/api/history")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => [],
        });
      }
      return prevImpl?.(url, init);
    });

    let deleteResult: boolean | undefined;
    await act(async () => {
      deleteResult = await result.current.deleteSession("s1");
    });

    expect(deleteResult).toBe(true);
    // Optimistic update in mutation onSuccess removes session from cache
    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(0);
      },
      { timeout: 1000 },
    );
  });

  it("deleteSession returns false on non-ok response", async () => {
    setupFetch();

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (init?.method === "DELETE") {
        return Promise.resolve({ ok: false, status: 500 });
      }
      return Promise.resolve({ ok: false });
    });

    let deleteResult: boolean | undefined;
    await act(async () => {
      deleteResult = await result.current.deleteSession("s1");
    });

    expect(deleteResult).toBe(false);
    expect(result.current.sessions).toHaveLength(1);
  });

  it("deleteSession returns false on network error", async () => {
    setupFetch();

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (init?.method === "DELETE") {
        return Promise.reject(new Error("Network error"));
      }
      return Promise.resolve({ ok: false });
    });

    let deleteResult: boolean | undefined;
    await act(async () => {
      deleteResult = await result.current.deleteSession("s1");
    });

    expect(deleteResult).toBe(false);
    expect(result.current.sessions).toHaveLength(1);
  });

  it("keeps previous sessions visible while date filter refetches", async () => {
    setupFetch();

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );

    const previousSession = result.current.sessions[0];

    // Switch to a date with no data, but use a slow mock to check intermediate state
    const slowPromise = new Promise((resolve) => {
      setTimeout(() => {
        resolve({ status: 204, ok: false });
      }, 200);
    });

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => mockVehicles });
      }
      if (url.includes("/api/history")) {
        return slowPromise;
      }
      return Promise.resolve({ ok: false });
    });

    await act(async () => {
      result.current.setSelectedDate("2025-06-01");
    });

    // While the new query is still fetching, old sessions should remain visible
    expect(result.current.sessions).toContainEqual(previousSession);

    // Wait for the slow fetch to complete
    await act(async () => {
      await new Promise((r) => setTimeout(r, 300));
    });

    // After fetch completes with no data, sessions should be empty
    expect(result.current.sessions).toHaveLength(0);
  });

  it("keeps previous sessions visible while vehicle filter refetches", async () => {
    const sm = [
      cloneSession({ vehicleId: "v1" }),
      cloneSession({ id: "s2", vehicleId: "v2" }),
    ];
    setupFetch(mockVehicles, sm);

    const { result } = customRenderHook(() => useHistory());

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(2);
      },
      { timeout: 1000 },
    );

    const previousSessions = [...result.current.sessions];

    // Switch vehicle filter with a slow mock
    const slowPromise = new Promise((resolve) => {
      setTimeout(() => {
        resolve({
          ok: true,
          json: async () => sm.filter((s) => s.vehicleId === "v1"),
        });
      }, 200);
    });

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({ ok: true, json: async () => mockVehicles });
      }
      if (url.includes("/api/history")) {
        return slowPromise;
      }
      return Promise.resolve({ ok: false });
    });

    await act(async () => {
      result.current.handleVehicleChange("v1");
    });

    // While fetching, old sessions should remain visible
    expect(result.current.sessions).toHaveLength(2);
    expect(result.current.sessions).toEqual(previousSessions);

    // Wait for fetch to complete
    await act(async () => {
      await new Promise((r) => setTimeout(r, 300));
    });

    await waitFor(
      () => {
        expect(result.current.sessions).toHaveLength(1);
      },
      { timeout: 1000 },
    );
    expect(result.current.sessions[0]?.vehicleId).toBe("v1");
  });

  it("initializes with SSR data without fetching", () => {
    const ssrVehicles = mockVehicles as HistoryVehicle[];
    const ssrSessions = mockSessions as HistoryChargeSession[];

    const { result } = customRenderHook(() =>
      useHistory({
        initialVehicles: ssrVehicles,
        initialSessions: ssrSessions,
      }),
    );

    expect(result.current.loading).toBe(false);
    expect(result.current.vehicles).toHaveLength(1);
    expect(result.current.sessions).toHaveLength(1);
    expect(result.current.error).toBeNull();
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
