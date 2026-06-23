import { customRenderHook as renderHook, act, waitFor } from "@/test-utils";
import { describe, it, expect, vi, beforeEach } from "vitest";

import { useSchedule } from "./useSchedule";

const TEST_PLUG_ID = "test-plug-id";

function createResponse(opts: {
  ok?: boolean;
  status?: number;
  data?: unknown;
}) {
  const { ok = true, status = opts.ok ? 200 : 500, data = null } = opts;
  return {
    ok,
    status: status ?? (ok ? 200 : 500),
    json: () => Promise.resolve(data),
  };
}

describe("useSchedule", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches schedule on mount", async () => {
    const mockSchedule = {
      id: "plug",
      type: "daily",
      time: "03:00",
      enabled: true,
    };
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(createResponse({ data: mockSchedule })),
    );

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.schedule).toEqual(mockSchedule);
    });
  });

  it("does not fetch when plugId is null", async () => {
    const mockFetch = vi.fn();
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useSchedule(null));

    expect(result.current.isLoading).toBe(false);
    expect(result.current.schedule).toBeNull();
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("sets schedule to null when backend returns 404", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(createResponse({ ok: false, status: 404 })),
    );

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
    });

    expect(result.current.schedule).toBeNull();
  });

  it("sets schedule to null on fetch error", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new Error("Network error")),
    );

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
    });

    expect(result.current.schedule).toBeNull();
  });

  it("saveSchedule patches and updates local state", async () => {
    const mockSchedule = {
      id: "plug",
      type: "daily",
      time: "05:00",
      enabled: true,
    };
    const mockFetch = vi
      .fn()
      .mockResolvedValueOnce(createResponse({ ok: false, status: 404 }))
      .mockResolvedValueOnce(createResponse({ data: mockSchedule }));
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let saved: any;
    await act(async () => {
      saved = await result.current.saveSchedule({
        type: "daily",
        time: "05:00",
        enabled: true,
      });
    });

    expect(saved).toEqual(mockSchedule);

    await waitFor(() => {
      expect(result.current.schedule).toEqual(mockSchedule);
    });
  });

  it("saveSchedule returns null on network error", async () => {
    const mockFetch = vi
      .fn()
      .mockResolvedValue(createResponse({ ok: false, status: 404 }))
      .mockRejectedValueOnce(new Error("Network error"));
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let saved: any;
    await act(async () => {
      saved = await result.current.saveSchedule({
        type: "daily",
        time: "03:00",
        enabled: true,
      });
    });

    expect(saved).toBeNull();
  });

  it("saveSchedule calls correct per-plug endpoint", async () => {
    const mockSchedule = {
      id: "plug",
      type: "daily",
      time: "04:00",
      enabled: false,
    };
    const mockFetch = vi
      .fn()
      .mockResolvedValueOnce(createResponse({ ok: false, status: 404 }))
      .mockResolvedValueOnce(createResponse({ data: mockSchedule }));
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.saveSchedule({
        type: "daily",
        time: "04:00",
        enabled: false,
      });
    });

    const patchCall = mockFetch.mock.calls[1] as [string, RequestInit];
    expect(patchCall[0]).toBe(`/api/plugs/${TEST_PLUG_ID}/schedule`);
    expect(patchCall[1]).toEqual({
      method: "PATCH",
      cache: "no-store",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ type: "daily", time: "04:00", enabled: false }),
    });
  });

  it("initializes with SSR data without fetching", async () => {
    const mockFetch = vi.fn();
    vi.stubGlobal("fetch", mockFetch);

    const ssrSchedule = {
      id: "plug",
      type: "daily" as const,
      time: "03:00",
      enabled: true,
    };

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID, ssrSchedule));

    expect(result.current.isLoading).toBe(false);
    expect(result.current.schedule).toEqual(ssrSchedule);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("initializes with SSR data and renderTimeMs without fetching", async () => {
    const mockFetch = vi.fn();
    vi.stubGlobal("fetch", mockFetch);

    const ssrSchedule = {
      id: "plug",
      type: "daily" as const,
      time: "07:00",
      enabled: false,
    };
    const renderTimeMs = Date.now();

    const { result } = renderHook(() =>
      useSchedule(TEST_PLUG_ID, ssrSchedule, renderTimeMs),
    );

    expect(result.current.isLoading).toBe(false);
    expect(result.current.schedule).toEqual(ssrSchedule);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("fetches when no SSR data provided", async () => {
    const mockSchedule = {
      id: "plug",
      type: "daily",
      time: "03:00",
      enabled: true,
    };
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(createResponse({ data: mockSchedule })),
    );

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.schedule).toEqual(mockSchedule);
    });
  });

  it("validates response with Zod schema on fetch", async () => {
    const invalidSchedule = { id: "plug", time: "not-a-time", enabled: "yes" };
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(createResponse({ data: invalidSchedule })),
    );

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
    });

    expect(result.current.schedule).toBeNull();
  });

  it("validates response with Zod schema on save", async () => {
    const mockSchedule = {
      id: "plug",
      type: "daily",
      time: "03:00",
      enabled: true,
    };
    const invalidResponse = { id: "plug", enabled: true }; // Missing 'time'
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(createResponse({ data: mockSchedule }))
        .mockResolvedValueOnce(createResponse({ data: invalidResponse })),
    );

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.schedule).toEqual(mockSchedule);
    });

    let saved: any;
    await act(async () => {
      saved = await result.current.saveSchedule({
        type: "daily",
        time: "04:00",
        enabled: false,
      });
    });

    expect(saved).toBeNull();
  });

  it("saveSchedule carbon_aware payload includes window fields", async () => {
    const mockSchedule = {
      id: "plug",
      type: "carbon_aware",
      time: "22:00",
      windowStart: "22:00",
      windowEnd: "06:00",
      enabled: true,
    };
    const mockFetch = vi
      .fn()
      .mockResolvedValueOnce(createResponse({ ok: false, status: 404 }))
      .mockResolvedValueOnce(createResponse({ data: mockSchedule }));
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useSchedule(TEST_PLUG_ID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.saveSchedule({
        type: "carbon_aware",
        windowStart: "22:00",
        windowEnd: "06:00",
        enabled: true,
      });
    });

    const patchCall = mockFetch.mock.calls[1] as [string, RequestInit];
    expect(patchCall[1]).toMatchObject({
      body: JSON.stringify({
        type: "carbon_aware",
        windowStart: "22:00",
        windowEnd: "06:00",
        enabled: true,
      }),
    });
  });
});
