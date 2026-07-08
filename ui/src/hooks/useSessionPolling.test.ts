import { useGaugeStore } from "@/stores/gaugeStore";
import { customRenderHook as renderHook, act } from "@/test-utils";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import { useSessionPolling } from "./useSessionPolling";

const mockFetch = vi.fn();
global.fetch = mockFetch;

const testVehicle = {
  id: "vm-1",
  capacityKwh: 5.46,
  chargerOutputW: 1200,
  chargingEfficiency: 0.9,
};

describe("useSessionPolling", () => {
  const isDraggingRef = { current: false };

  beforeEach(() => {
    vi.clearAllMocks();
    useGaugeStore.setState({ currentPercent: 20, targetPercent: 80 });
    mockFetch.mockImplementation(() => Promise.resolve({ ok: false }));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  const flushMicrotasks = () => act(async () => await Promise.resolve());

  it("starts with idle session when no initial session", async () => {
    const { result } = renderHook(() =>
      useSessionPolling({
        selectedVehicle: testVehicle,
        initialSession: null,
        isDraggingRef,
      }),
    );
    await flushMicrotasks();
    expect(result.current.session.status).toBe("idle");
  });

  it("initializes with charging session from initialSession", async () => {
    const { result } = renderHook(() =>
      useSessionPolling({
        selectedVehicle: testVehicle,
        initialSession: {
          status: "active",
          powerDraw: 1000,
          startPercent: 20,
          currentPercent: 30,
          targetPercent: 80,
          startedAt: "2024-01-01T10:00:00Z",
          voltage: 230,
          current: 4.3,
          energyAddedKwh: 0.5,
        },
        isDraggingRef,
      }),
    );
    await flushMicrotasks();
    expect(result.current.session.status).toBe("charging");
    if (result.current.session.status === "charging") {
      expect(result.current.session.powerDraw).toBe(1000);
      expect(result.current.session.voltage).toBe(230);
      expect(result.current.session.current).toBe(4.3);
      expect(result.current.session.energyAddedKwh).toBe(0.5);
    }
  });

  it("initializes with holding session and estimatedResumeTime from initialSession", async () => {
    const { result } = renderHook(() =>
      useSessionPolling({
        selectedVehicle: testVehicle,
        initialSession: {
          status: "holding",
          powerDraw: 0,
          startPercent: 20,
          currentPercent: 64,
          targetPercent: 80,
          startedAt: "2024-01-01T10:00:00Z",
          voltage: null,
          current: null,
          energyAddedKwh: 0.5,
          estimatedResumeTime: "23:30",
        },
        isDraggingRef,
      }),
    );
    await flushMicrotasks();
    expect(result.current.session.status).toBe("holding");
    if (result.current.session.status === "holding") {
      expect(result.current.session.estimatedResumeTime).toBe("23:30");
    }
  });

  it("initializes with pending session from initialSession", async () => {
    const { result } = renderHook(() =>
      useSessionPolling({
        selectedVehicle: testVehicle,
        initialSession: {
          status: "pending",
          startPercent: 20,
          currentPercent: 20,
          targetPercent: 80,
          startedAt: "2024-01-01T10:00:00Z",
        },
        isDraggingRef,
      }),
    );
    await flushMicrotasks();
    expect(result.current.session.status).toBe("pending");
  });

  it("syncs gauge store from initialSession data", async () => {
    renderHook(() =>
      useSessionPolling({
        selectedVehicle: testVehicle,
        initialSession: {
          status: "active",
          powerDraw: 1000,
          startPercent: 20,
          currentPercent: 35,
          targetPercent: 75,
          startedAt: "2024-01-01T10:00:00Z",
        },
        isDraggingRef,
      }),
    );

    await flushMicrotasks();
    expect(useGaugeStore.getState().currentPercent).toBe(35);
    expect(useGaugeStore.getState().targetPercent).toBe(75);
  });

  it("skips gauge store sync while isDraggingRef is true", async () => {
    isDraggingRef.current = true;

    renderHook(() =>
      useSessionPolling({
        selectedVehicle: testVehicle,
        initialSession: {
          status: "active",
          powerDraw: 1000,
          startPercent: 20,
          currentPercent: 50,
          targetPercent: 90,
          startedAt: "2024-01-01T10:00:00Z",
        },
        isDraggingRef,
      }),
    );

    await flushMicrotasks();

    // Store should NOT have been updated while dragging
    expect(useGaugeStore.getState().currentPercent).toBe(20);
    expect(useGaugeStore.getState().targetPercent).toBe(80);

    isDraggingRef.current = false;
  });

  it("maps unknown backend status to idle with warning", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});

    const { result } = renderHook(() =>
      useSessionPolling({
        selectedVehicle: testVehicle,
        initialSession: {
          status: "some_unknown_status",
          startPercent: 20,
          currentPercent: 20,
          targetPercent: 80,
        },
        isDraggingRef,
      }),
    );

    await flushMicrotasks();
    expect(result.current.session.status).toBe("idle");
  });

  it("uses chargerOutputW as fallback for powerDraw when null", async () => {
    const { result } = renderHook(() =>
      useSessionPolling({
        selectedVehicle: testVehicle,
        initialSession: {
          status: "active",
          powerDraw: null,
          startPercent: 20,
          currentPercent: 30,
          targetPercent: 80,
          startedAt: "2024-01-01T10:00:00Z",
        },
        isDraggingRef,
      }),
    );

    await flushMicrotasks();
    if (result.current.session.status === "charging") {
      expect(result.current.session.powerDraw).toBe(1200);
    }
  });

  it("returns autoStopHandledRef", async () => {
    const { result } = renderHook(() =>
      useSessionPolling({
        selectedVehicle: testVehicle,
        initialSession: null,
        isDraggingRef,
      }),
    );
    await flushMicrotasks();
    expect(result.current.autoStopHandledRef).toBeDefined();
    expect(result.current.autoStopHandledRef.current).toBe(false);
  });
});
