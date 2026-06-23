import { customRender, render, screen, waitFor, act } from "@/test-utils";
import { createSOCSnapshot } from "@/test/fixtures";
import { describe, it, expect, vi, beforeEach } from "vitest";

import SocChart from "./SocChart";
import { renderSocTooltip } from "./SocTooltip";

describe("SocChart", () => {
  const mockFetch = vi.fn();

  beforeEach(() => {
    vi.stubGlobal("fetch", mockFetch);
    mockFetch.mockClear();
  });

  it("shows loading state initially", () => {
    mockFetch.mockImplementation(() => new Promise(() => {}));
    customRender(<SocChart />);
    expect(screen.getByText(/loading soc data/i)).toBeInTheDocument();
  });

  it("shows no active session when API returns 204", async () => {
    mockFetch.mockResolvedValueOnce({ status: 204, ok: false });
    customRender(<SocChart />);
    await waitFor(() => {
      expect(screen.getByText(/no active charge session/i)).toBeInTheDocument();
    });
  });

  it("renders chart with a single snapshot", async () => {
    const mockSnapshots = [
      createSOCSnapshot({
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        socPercent: 20,
      }),
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockSnapshots,
    });
    customRender(<SocChart />);
    await waitFor(() => {
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
  });

  it("renders chart when snapshots are available", async () => {
    const mockSnapshots = [
      createSOCSnapshot({
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        socPercent: 20,
      }),
      createSOCSnapshot({
        id: "2",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:05Z",
        socPercent: 21,
      }),
      createSOCSnapshot({
        id: "3",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:10Z",
        socPercent: 22,
      }),
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockSnapshots,
    });
    customRender(<SocChart />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/soc-snapshots",
        expect.any(Object),
      );
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
  });

  it("polls every 5 seconds", async () => {
    const mockSnapshots = [
      createSOCSnapshot({
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        socPercent: 20,
      }),
    ];
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockSnapshots,
    });
    customRender(<SocChart />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 5100));
    });
    expect(mockFetch).toHaveBeenCalledTimes(2);
  }, 10000);

  it("fetches with sessionId prop instead of polling", async () => {
    const mockSnapshots = [
      createSOCSnapshot({
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        socPercent: 20,
      }),
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockSnapshots,
    });
    customRender(<SocChart sessionId="test-session-123" />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/soc-snapshots?sessionId=test-session-123",
        expect.any(Object),
      );
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 5100));
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  }, 10000);

  it("does not poll when shouldPoll is false", async () => {
    const mockSnapshots = [
      createSOCSnapshot({
        id: "1",
        sessionId: "s1",
        timestamp: "2024-01-01T10:00:00Z",
        socPercent: 20,
      }),
    ];
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockSnapshots,
    });
    customRender(<SocChart shouldPoll={false} />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 5100));
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  }, 10000);
});

describe("renderSocTooltip", () => {
  it("renders SOC percentage", () => {
    const { getByText } = render(renderSocTooltip(45.6, "10:30"));
    expect(getByText("45.60%")).toBeInTheDocument();
  });

  it("renders timestamp", () => {
    const { getByText } = render(renderSocTooltip(45.6, "10:30"));
    expect(getByText("10:30")).toBeInTheDocument();
  });
});
