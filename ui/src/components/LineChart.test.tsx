import { customRender, screen, waitFor, act } from "@/test-utils";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { z } from "zod";

import LineChart, { LineChartProps } from "./LineChart";

const TestDataSchema = z.object({
  id: z.string(),
  ts: z.string(),
  val: z.number(),
});
type TestData = z.infer<typeof TestDataSchema>;

const mockData: TestData[] = [
  { id: "1", ts: "2024-01-01T10:00:00Z", val: 10 },
  { id: "2", ts: "2024-01-01T10:00:05Z", val: 20 },
  { id: "3", ts: "2024-01-01T10:00:10Z", val: 30 },
];

function defaultProps(
  extra?: Partial<LineChartProps<TestData>>,
): LineChartProps<TestData> {
  return {
    fetchConfig: { endpoint: "/api/test", pollingIntervalMs: 5000 },
    schema: TestDataSchema,
    yExtractor: (d) => d.val,
    timestampExtractor: (d) => {
      const date = new Date(d.ts);
      return date.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      });
    },
    yDomain: (points) => {
      const vals = points.map((p) => p.val);
      return [Math.min(...vals) - 2, Math.max(...vals) + 2];
    },
    lineColor: "#3b82f6",
    minPointsForRender: 3,
    heightPx: 160,
    ariaLabel: "Test chart",
    ...extra,
  };
}

describe("LineChart", () => {
  const mockFetch = vi.fn();

  beforeEach(() => {
    mockFetch.mockClear();
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockData,
    });
    const gt = globalThis as Record<string, unknown>;
    gt.fetch = mockFetch;
    vi.stubGlobal("localStorage", {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
    });
  });

  afterEach(() => {
    const gt = globalThis as Record<string, unknown>;
    delete gt.fetch;
    vi.unstubAllGlobals();
  });

  // --- Placeholder states ---

  it("shows loading state initially", () => {
    mockFetch.mockImplementation(() => new Promise(() => {}));
    customRender(<LineChart {...defaultProps()} />);
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it("shows empty state when API returns 204", async () => {
    mockFetch.mockResolvedValueOnce({ status: 204, ok: false });
    customRender(<LineChart {...defaultProps()} />);
    await waitFor(() => {
      expect(screen.getByText(/no active charge session/i)).toBeInTheDocument();
    });
  });

  it("shows waiting state when fewer than minPointsForRender", async () => {
    const sparseData = [
      { id: "1", ts: "2024-01-01T10:00:00Z", val: 10 },
      { id: "2", ts: "2024-01-01T10:00:05Z", val: 20 },
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => sparseData,
    });
    customRender(<LineChart {...defaultProps()} />);
    await waitFor(() => {
      expect(
        screen.getByText(/not enough data to display/i),
      ).toBeInTheDocument();
    });
  });

  it("uses custom messages for all states", async () => {
    const msgs = {
      loading: "Fetching...",
      empty: "Nothing here",
      waiting: "Not yet...",
    };
    mockFetch.mockResolvedValueOnce({ status: 204, ok: false });
    customRender(<LineChart {...defaultProps()} messages={msgs} />);
    await waitFor(() => {
      expect(screen.getByText(/nothing here/i)).toBeInTheDocument();
    });

    const sparseData = [
      { id: "1", ts: "2024-01-01T10:00:00Z", val: 10 },
      { id: "2", ts: "2024-01-01T10:00:05Z", val: 20 },
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => sparseData,
    });
    customRender(<LineChart {...defaultProps()} messages={msgs} />);
    await waitFor(() => {
      expect(screen.getByText(/not yet/i)).toBeInTheDocument();
    });
  });

  // --- Rendering ---

  it("renders chart container when data is available", async () => {
    customRender(<LineChart {...defaultProps()} />);
    await waitFor(() => {
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
  });

  it("renders chart with a single data point (filler extends to current time)", async () => {
    const singlePoint = [{ id: "1", ts: "2024-01-01T10:00:00Z", val: 42 }];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => singlePoint,
    });
    customRender(<LineChart {...defaultProps({ minPointsForRender: 1 })} />);
    await waitFor(() => {
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
    // "Not enough data" should not appear - the chart renders with the filler point.
    expect(screen.queryByText(/not enough data/i)).not.toBeInTheDocument();
  });

  // --- Data fetching ---

  it("polls at configured interval", async () => {
    customRender(
      <LineChart
        {...defaultProps({
          fetchConfig: { endpoint: "/api/test", pollingIntervalMs: 5000 },
        })}
      />,
    );
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 5100));
    });
    expect(mockFetch).toHaveBeenCalledTimes(2);
  }, 10000);

  it("uses NEXT_PUBLIC_API_URL env var when set", async () => {
    const orig = process.env.NEXT_PUBLIC_API_URL;
    process.env.NEXT_PUBLIC_API_URL = "http://custom-api.local";
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockData,
    });
    customRender(<LineChart {...defaultProps()} />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith("/api/test", expect.any(Object));
    });
    process.env.NEXT_PUBLIC_API_URL = orig;
  });

  // --- Sync callbacks ---

  it("calls onSync with data when loaded", async () => {
    const onSync = vi.fn();
    customRender(<LineChart {...defaultProps()} onSync={onSync} />);
    await waitFor(() => {
      const dataLoadedCall = onSync.mock.calls.find(
        (call) => (call[0] as TestData[]).length === 3,
      );
      expect(dataLoadedCall).toBeDefined();
    });
    const dataLoadedCall = onSync.mock.calls.find(
      (call) => (call[0] as TestData[]).length === 3,
    );
    const data = (dataLoadedCall ?? [])[0] as TestData[];
    expect(data).toHaveLength(3);
  });

  it("calls onSync with no-data reason when API returns 204", async () => {
    const onSync = vi.fn();
    mockFetch.mockResolvedValue({ status: 204, ok: false });
    customRender(<LineChart {...defaultProps()} onSync={onSync} />);
    await act(async () => {
      await new Promise((r) => setTimeout(r, 100));
    });
    const lastCall = onSync.mock.calls[onSync.mock.calls.length - 1];
    const reason = (lastCall ?? [, ,])[2] as string | undefined;
    expect(reason).toBe("no-data");
  });

  it("calls onSync with few-points reason when data is insufficient", async () => {
    const onSync = vi.fn();
    const sparseData = [
      { id: "1", ts: "2024-01-01T10:00:00Z", val: 10 },
      { id: "2", ts: "2024-01-01T10:00:05Z", val: 20 },
    ];
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => sparseData,
    });
    customRender(<LineChart {...defaultProps()} onSync={onSync} />);
    await waitFor(() => {
      const fewPointsCall = onSync.mock.calls.find(
        (call) => (call[2] as string | undefined) === "few-points",
      );
      expect(fewPointsCall).toBeDefined();
    });
  });

  // --- Customization ---

  it("applies custom height", async () => {
    customRender(<LineChart {...defaultProps({ heightPx: 200 })} />);
    await waitFor(() => {
      const container = document.querySelector(".rounded-lg.overflow-hidden");
      expect(container).toBeInTheDocument();
    });
  });

  it("defaults to 160px height when heightPx is undefined", async () => {
    customRender(<LineChart {...defaultProps({ heightPx: undefined })} />);
    await waitFor(() => {
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
  });

  // --- Edge cases ---

  it("handles fetch error gracefully (shows empty state)", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));
    customRender(<LineChart {...defaultProps()} />);
    await waitFor(() => {
      expect(screen.getByText(/no active charge session/i)).toBeInTheDocument();
    });
  });

  it("shows empty state on non-ok HTTP response", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => [],
    });
    customRender(<LineChart {...defaultProps()} />);
    await waitFor(() => {
      expect(screen.getByText(/no active charge session/i)).toBeInTheDocument();
    });
  });

  it("includes vehicleId in fetch URL when provided as prop", async () => {
    const orig = process.env.NEXT_PUBLIC_API_URL;
    process.env.NEXT_PUBLIC_API_URL = "http://localhost:8080";
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockData,
    });
    customRender(<LineChart {...defaultProps({ vehicleId: "my-vehicle" })} />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/test?vehicleId=my-vehicle",
        expect.any(Object),
      );
    });
    process.env.NEXT_PUBLIC_API_URL = orig;
  });

  it("omits vehicleId from URL when prop is not provided", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => mockData,
    });
    customRender(<LineChart {...defaultProps()} />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith("/api/test", expect.any(Object));
    });
  });

  it("renders without fetching when staticData is provided", async () => {
    customRender(<LineChart {...defaultProps({ staticData: mockData })} />);
    await waitFor(() => {
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("fetches only once when shouldPoll is false", async () => {
    customRender(
      <LineChart
        {...defaultProps({
          shouldPoll: false,
          fetchConfig: { endpoint: "/api/test", pollingIntervalMs: 5000 },
        })}
      />,
    );
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 5100));
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  }, 10000);

  it("accepts yFormatter prop for custom axis label formatting", async () => {
    const yFmt = vi.fn((v: number) => `${v} kW`);
    customRender(
      <LineChart
        {...defaultProps({ yFormatter: yFmt, staticData: mockData })}
      />,
    );
    await waitFor(() => {
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
  });

  it("renders tooltip element when rendering tooltip content", async () => {
    const tooltipRender = vi.fn((_value: number, _time: string) => (
      <div className="tooltip" data-testid="tooltip">
        Test tooltip
      </div>
    ));
    customRender(
      <LineChart
        {...defaultProps({
          renderTooltipContent: tooltipRender,
          staticData: mockData,
        })}
      />,
    );

    await waitFor(() => {
      expect(
        document.querySelector(".recharts-responsive-container"),
      ).toBeInTheDocument();
    });
  });
});
