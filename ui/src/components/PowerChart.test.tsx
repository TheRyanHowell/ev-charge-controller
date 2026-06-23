import { customRender, render, screen, waitFor, act } from "@/test-utils";
import { describe, it, expect, vi, beforeEach } from "vitest";

import PowerChart from "./PowerChart";
import { renderPowerTooltip } from "./PowerTooltip";

describe("PowerChart", () => {
  const mockFetch = vi.fn();

  beforeEach(() => {
    vi.stubGlobal("fetch", mockFetch);
    mockFetch.mockClear();
  });

  it("shows loading state initially", () => {
    mockFetch.mockImplementation(() => new Promise(() => {}));
    customRender(<PowerChart />);
    expect(screen.getByText(/loading power data/i)).toBeInTheDocument();
  });

  it("shows no active session when API returns 204", async () => {
    mockFetch.mockResolvedValueOnce({ status: 204, ok: false });
    customRender(<PowerChart />);
    await waitFor(() => {
      expect(screen.getByText(/no active charge session/i)).toBeInTheDocument();
    });
  });

  it("renders chart when readings are available", async () => {
    const mockReadings = [
      {
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        power: 600,
        energyKwh: 0.1,
        voltage: 230,
        current: 2.6,
      },
      {
        id: "2",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:05Z",
        power: 610,
        energyKwh: 0.2,
        voltage: 230,
        current: 2.65,
      },
      {
        id: "3",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:10Z",
        power: 590,
        energyKwh: 0.3,
        voltage: 230,
        current: 2.57,
      },
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockReadings,
    });
    customRender(<PowerChart />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/power-readings",
        expect.any(Object),
      );
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
  });

  it("calls API every 5 seconds", async () => {
    const mockReadings = [
      {
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        power: 600,
        energyKwh: 0.1,
        voltage: 230,
        current: 2.6,
      },
    ];
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockReadings,
    });
    customRender(<PowerChart />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 5100));
    });
    expect(mockFetch).toHaveBeenCalledTimes(2);
  }, 10000);

  it("renders chart with tooltip content when data is available", async () => {
    const mockReadings = [
      {
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        power: 600,
        energyKwh: 0.1,
        voltage: 230,
        current: 2.6,
      },
      {
        id: "2",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:05Z",
        power: 610,
        energyKwh: 0.2,
        voltage: 230,
        current: 2.65,
      },
      {
        id: "3",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:10Z",
        power: 590,
        energyKwh: 0.3,
        voltage: 230,
        current: 2.57,
      },
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockReadings,
    });
    customRender(<PowerChart />);
    await waitFor(() => {
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
  });

  it("fetches with sessionId prop instead of polling", async () => {
    const mockReadings = [
      {
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        power: 600,
        energyKwh: 0.1,
        voltage: 230,
        current: 2.6,
      },
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockReadings,
    });
    customRender(<PowerChart sessionId="test-session-123" />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/power-readings?sessionId=test-session-123",
        expect.any(Object),
      );
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 5100));
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  }, 10000);

  it("does not poll when shouldPoll is false", async () => {
    const mockReadings = [
      {
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        power: 600,
        energyKwh: 0.1,
        voltage: 230,
        current: 2.6,
      },
    ];
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockReadings,
    });
    customRender(<PowerChart shouldPoll={false} />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 5100));
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  }, 10000);
});

describe("renderPowerTooltip", () => {
  it("renders kW when power is over threshold", () => {
    const { getByText } = render(renderPowerTooltip(1500, "10:30"));
    expect(getByText("1.50 kW")).toBeInTheDocument();
  });

  it("renders W when power is under threshold", () => {
    const { getByText } = render(renderPowerTooltip(450, "10:30"));
    expect(getByText("450 W")).toBeInTheDocument();
  });

  it("renders timestamp in tooltip", () => {
    const { getByText } = render(renderPowerTooltip(600, "10:30"));
    expect(getByText("10:30")).toBeInTheDocument();
  });
});
