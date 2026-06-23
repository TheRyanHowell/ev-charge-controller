import type { Plug } from "@/lib/schemas";

import { queryKeys } from "@/lib/queryKeys";
import { customRenderHook as renderHook, waitFor, act } from "@/test-utils";
import { createPlug } from "@/test/fixtures";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook as rtlRenderHook } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import { usePlug } from "./usePlug";

const mockFetch = vi.fn();

// Plugs assigned to vehicle "vehicle-1" and "vehicle-2".
const plugWithVehicle1 = createPlug({
  id: "plug-1",
  namespace: "ns-test",
  lastSeen: null,
  vehicleId: "vehicle-1",
  type: "charging",
});

const plugWithVehicle2 = createPlug({
  id: "plug-2",
  name: "Plug 2",
  namespace: "ns-test2",
  mqttTopic: "test/topic2",
  lastSeen: null,
  vehicleId: "vehicle-2",
  type: "charging",
});

describe("usePlug (vehicle-centric)", () => {
  const plugs: Plug[] = [plugWithVehicle1];

  beforeEach(() => {
    vi.stubGlobal("fetch", mockFetch);
    mockFetch.mockReset();
    mockFetch.mockImplementation(() =>
      Promise.resolve({ ok: true, json: async () => plugs }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("fetches plugs and returns them", async () => {
    const { result } = renderHook(() => usePlug());
    await waitFor(() => {
      expect(result.current.plugs).toHaveLength(1);
    });
    expect(result.current.plugs[0]?.id).toBe("plug-1");
  });

  it("defaults selectedVehicleId to vehicle of first plug when initialPlugs provided", () => {
    const { result } = renderHook(() => usePlug(plugs));
    // First plug's vehicleId becomes selectedVehicleId
    expect(result.current.selectedVehicleId).toBe("vehicle-1");
  });

  it("starts with null selectedVehicleId when no initial plugs", async () => {
    const { result } = renderHook(() => usePlug());
    // Before fetch completes, selectedVehicleId is null
    expect(result.current.selectedVehicleId).toBeNull();
  });

  it("uses initialSelectedVehicleId when provided", () => {
    const { result } = renderHook(() =>
      usePlug(plugs, Date.now(), "vehicle-1"),
    );
    expect(result.current.selectedVehicleId).toBe("vehicle-1");
  });

  it("returns isLoading true initially without initialPlugs", () => {
    const { result } = renderHook(() => usePlug());
    expect(result.current.isLoading).toBe(true);
  });

  it("allows selecting a different vehicle", async () => {
    const twoPlugs: Plug[] = [plugWithVehicle1, plugWithVehicle2];
    mockFetch.mockImplementation(() =>
      Promise.resolve({ ok: true, json: async () => twoPlugs }),
    );

    const { result } = renderHook(() => usePlug());
    await waitFor(() => {
      expect(result.current.plugs).toHaveLength(2);
    });

    await act(async () => {
      result.current.selectVehicle("vehicle-2");
    });
    expect(result.current.selectedVehicleId).toBe("vehicle-2");
  });

  it("accepts initialPlugs and initialDataUpdatedAt", async () => {
    const { result } = renderHook(() => usePlug(plugs, Date.now()));
    expect(result.current.plugs).toHaveLength(1);
    expect(result.current.plugs[0]?.id).toBe("plug-1");
    expect(result.current.isLoading).toBe(false);
  });

  it("returns null selectedVehicleId when no plugs have vehicleIds", async () => {
    const noVehiclePlugs = [
      createPlug({ namespace: "ns-x", lastSeen: null, vehicleId: null }),
    ];
    mockFetch.mockImplementation(() =>
      Promise.resolve({ ok: true, json: async () => noVehiclePlugs }),
    );
    const { result } = renderHook(() => usePlug(noVehiclePlugs));
    // No plug has a vehicleId so selectedVehicleId is null
    expect(result.current.selectedVehicleId).toBeNull();
  });

  it("handles fetch error", async () => {
    mockFetch.mockImplementation(() =>
      Promise.resolve({
        ok: false,
        status: 500,
        json: async () => ({ detail: "server error" }),
      }),
    );
    const { result } = renderHook(() => usePlug());
    await waitFor(() => {
      expect(result.current.error).toBeDefined();
    });
  });

  it("exposes mutation functions", async () => {
    const { result } = renderHook(() => usePlug());
    await waitFor(() => {
      expect(result.current.plugs).toHaveLength(1);
    });
    expect(typeof result.current.createPlug).toBe("function");
    expect(typeof result.current.updatePlug).toBe("function");
    expect(typeof result.current.deletePlug).toBe("function");
    expect(typeof result.current.selectVehicle).toBe("function");
    expect(typeof result.current.toggleMaintenancePower).toBe("function");
    expect(typeof result.current.isCreating).toBe("boolean");
  });

  it("handles network error when fetch throws", async () => {
    mockFetch.mockImplementation(() =>
      Promise.reject(new Error("network error")),
    );
    const { result } = renderHook(() => usePlug());
    await waitFor(() => {
      expect(result.current.error).toBeDefined();
    });
    expect(result.current.plugs).toEqual([]);
  });

  it("transitions from loading to loaded state", async () => {
    const { result } = renderHook(() => usePlug());
    expect(result.current.isLoading).toBe(true);
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });
    expect(result.current.plugs).toHaveLength(1);
    expect(result.current.error).toBeNull();
  });

  it("handles multiple plug updates sequentially", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    const { result } = rtlRenderHook(() => usePlug(), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      ),
    });
    await waitFor(() => {
      expect(result.current.plugs).toHaveLength(1);
    });

    await act(async () => {
      await result.current.updatePlug({
        plugId: "plug-1",
        name: "First Update",
      });
    });
    const afterFirst = queryClient.getQueryData(queryKeys.plugs.all) as Plug[];
    expect(afterFirst?.[0]?.name).toBe("First Update");

    await act(async () => {
      await result.current.updatePlug({
        plugId: "plug-1",
        vehicleId: "vehicle-99",
      });
    });
    const afterSecond = queryClient.getQueryData(queryKeys.plugs.all) as Plug[];
    expect(afterSecond?.[0]?.vehicleId).toBe("vehicle-99");
  });

  it("persists vehicle selection across re-renders", async () => {
    const { result, rerender } = renderHook(() => usePlug(plugs));
    await act(async () => {
      result.current.selectVehicle("vehicle-1");
    });
    rerender();
    expect(result.current.selectedVehicleId).toBe("vehicle-1");
  });

  it("handles empty plug data from API", async () => {
    mockFetch.mockImplementation(() =>
      Promise.resolve({ ok: true, json: async () => [] }),
    );
    const { result } = renderHook(() => usePlug());
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });
    expect(result.current.plugs).toEqual([]);
    expect(result.current.selectedVehicleId).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it("creates a new plug and selects its vehicle", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    const newPlug = createPlug({
      id: "new-plug",
      name: "New Plug",
      namespace: "ns-new",
      mqttTopic: "new/topic",
      online: false,
      initialized: false,
      lastSeen: null,
      vehicleId: "new-vehicle",
      type: "charging",
    });

    mockFetch.mockImplementation((url: unknown, init: unknown) => {
      const urlStr = typeof url === "string" ? url : String(url);
      const req = init as RequestInit | undefined;
      if (req?.method === "POST" && urlStr.includes("/api/plugs")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ plug: newPlug }),
        });
      }
      if (urlStr.includes("/api/plugs")) {
        return Promise.resolve({ ok: true, json: async () => plugs });
      }
      return Promise.resolve({ ok: true, json: async () => [] });
    });

    const { result } = rtlRenderHook(() => usePlug(), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      ),
    });
    await waitFor(() => {
      expect(result.current.plugs).toHaveLength(1);
    });

    await act(async () => {
      await result.current.createPlug({
        name: "New Plug",
        mqttTopic: "new/topic",
        vehicleId: "new-vehicle",
      });
    });

    const cache = queryClient.getQueryData(queryKeys.plugs.all) as Plug[];
    expect(cache).toHaveLength(2);
    // Hook should have selected the new vehicle
    expect(result.current.selectedVehicleId).toBe("new-vehicle");
  });

  it("deletes a plug and removes it from cache", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    let currentPlugs: Plug[] = [plugWithVehicle1, plugWithVehicle2];
    mockFetch.mockImplementation((url: unknown) => {
      const urlStr = typeof url === "string" ? url : String(url);
      if (urlStr.includes("/api/plugs")) {
        return Promise.resolve({ ok: true, json: async () => currentPlugs });
      }
      return Promise.resolve({ ok: true, json: async () => [] });
    });

    const { result } = rtlRenderHook(() => usePlug(), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      ),
    });
    await waitFor(() => {
      expect(result.current.plugs).toHaveLength(2);
    });

    currentPlugs = currentPlugs.filter((p) => p.id !== "plug-2");
    await act(async () => {
      await result.current.deletePlug("plug-2");
    });

    const cache = queryClient.getQueryData(queryKeys.plugs.all) as Plug[];
    expect(cache).toHaveLength(1);
    expect(cache[0]?.id).toBe("plug-1");
  });

  it("optimistically updates powerOn when toggling maintenance power", async () => {
    const maintenancePlug = createPlug({
      id: "maint-1",
      name: "12V",
      namespace: "ns-maint",
      lastSeen: null,
      vehicleId: "vehicle-1",
      type: "maintenance",
      powerOn: false,
    });
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    mockFetch.mockImplementation((url: unknown, init: unknown) => {
      const urlStr = typeof url === "string" ? url : String(url);
      const req = init as RequestInit | undefined;
      if (req?.method === "PATCH" && urlStr.includes("/power")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ ...maintenancePlug, powerOn: true }),
        });
      }
      return Promise.resolve({ ok: true, json: async () => [maintenancePlug] });
    });

    const { result } = rtlRenderHook(() => usePlug([maintenancePlug]), {
      wrapper: ({ children }) => (
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      ),
    });

    queryClient.setQueryData(queryKeys.plugs.all, [maintenancePlug]);

    await act(async () => {
      void result.current.toggleMaintenancePower({
        plugId: "maint-1",
        on: true,
      });
    });

    // Optimistic update should be in cache immediately
    const cache = queryClient.getQueryData(queryKeys.plugs.all) as Plug[];
    const updatedPlug = cache.find((p) => p.id === "maint-1");
    expect(updatedPlug?.powerOn).toBe(true);
  });
});
