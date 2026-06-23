import { useHistory } from "@/hooks";
import {
  customRender,
  screen,
  fireEvent,
  act,
  cleanup,
  waitFor,
} from "@/test-utils";
import { createHistorySession, createHistoryVehicle } from "@/test/fixtures";
import { describe, it, expect, vi, beforeEach } from "vitest";

import HistoryClient from "./HistoryClient";

let expandState: Record<string, boolean> = {};

vi.mock("@/hooks", () => {
  const loadMoreMock = vi.fn();
  const deleteSessionMock = vi.fn();
  const useHistoryMock = vi.fn(() => ({
    sessions: [],
    vehicles: [],
    selectedVehicleId: null,
    selectedDate: "2024-01-15",
    setSelectedDate: vi.fn(),
    loading: false,
    error: null,
    hasMore: true,
    loadMore: loadMoreMock,
    handleVehicleChange: vi.fn(),
    toggleExpand: vi.fn((id: string) => {
      expandState[id] = !expandState[id];
    }),
    getVehicleName: vi.fn((id: string) => id),
    isExpanded: vi.fn((id?: string) => expandState[id ?? "session-1"] ?? false),
    deleteSession: deleteSessionMock,
  }));
  return { useHistory: useHistoryMock };
});

vi.mock("@/utils/history", () => ({
  formatDuration: vi.fn((_start: string, _end?: string) => "2h 30m"),
  formatTimeRange: vi.fn((_start: string, _end?: string) => "10:00 – 12:30"),
  getTotalEnergy: vi.fn((_s: unknown) => "2.85"),
  getStatusColor: vi.fn((status: string) =>
    status === "completed" ? "bg-green-500" : "bg-yellow-500",
  ),
  getStatusBadgeClass: vi.fn((status: string) => `badge-${status}`),
}));

const mockSessions = [
  createHistorySession({
    id: "session-1",
    vehicleId: "vehicle-1",
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

const mockVehicles = [
  createHistoryVehicle({
    id: "vehicle-1",
    name: "Maeving RM1S",
    capacityKwh: 3.8,
    rangeMinMi: 100,
    rangeMaxMi: 150,
  }),
];

describe("History Page", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    expandState = {};
  });

  function mockHook(
    opts: {
      loading?: boolean;
      sessions?: unknown[];
      vehicles?: unknown[];
      error?: string | null;
      hasMore?: boolean;
    } = {},
  ) {
    const loadMore = vi.fn();
    const deleteSession = vi.fn();
    (useHistory as any).mockImplementation(() => ({
      sessions: opts.sessions ?? mockSessions,
      vehicles: opts.vehicles ?? mockVehicles,
      selectedVehicleId: null,
      selectedDate: "2024-01-15",
      setSelectedDate: vi.fn(),
      loading: opts.loading ?? false,
      error: opts.error ?? null,
      hasMore: opts.hasMore ?? true,
      loadMore,
      handleVehicleChange: vi.fn(),
      toggleExpand: vi.fn((id: string) => {
        expandState[id] = !expandState[id];
      }),
      getVehicleName: vi.fn((id: string) => id),
      isExpanded: vi.fn(
        (id?: string) => expandState[id ?? "session-1"] ?? false,
      ),
      deleteSession,
    }));
    return { loadMore, deleteSession };
  }

  it("renders the page title", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText("Charge History")).toBeInTheDocument();
  });

  it("displays session count", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText("1 session")).toBeInTheDocument();
  });

  it("displays back link", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(
      screen.getByRole("link", { name: "Back to dashboard" }),
    ).toBeInTheDocument();
  });

  it("displays vehicle filter dropdown", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByRole("combobox")).toBeInTheDocument();
    expect(screen.getByText("All Vehicles")).toBeInTheDocument();
    const options = screen.getAllByRole("option");
    const vehicleOption = Array.from(options).find((opt) =>
      opt.textContent?.includes("Maeving"),
    );
    expect(vehicleOption).toBeInTheDocument();
  });

  it("displays session list when available", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const buttons = screen.getAllByRole("button");
    const sessionButtons = buttons.filter(
      (btn) => btn.getAttribute("aria-expanded") === "false",
    );
    expect(sessionButtons.length).toBeGreaterThan(0);
  });

  it("displays session status", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText("completed")).toBeInTheDocument();
  });

  it("displays time range in session card", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(
      screen.getByText(
        (content) => content.includes("10:00") && content.includes("12:30"),
      ),
    ).toBeInTheDocument();
  });

  it("displays energy added", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText("+2.85 kWh")).toBeInTheDocument();
  });

  it("displays start and end percentages", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText("20%")).toBeInTheDocument();
    expect(screen.getByText("80%")).toBeInTheDocument();
  });

  it("displays no sessions message when empty", async () => {
    mockHook({ sessions: [] });
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText("No charge sessions yet")).toBeInTheDocument();
  });

  it("shows loading spinner while loading is true", async () => {
    mockHook({ loading: true });
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("expands session card on click", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const cardButton = screen.getByRole("button", { expanded: false });
    fireEvent.click(cardButton);
    // Verify expandState was toggled
    expect(expandState["session-1"]).toBe(true);
    // Re-render to pick up new isExpanded value from mock
    await act(async () => {
      cleanup();
      customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    });
    const updatedButton = screen.getByRole("button", { expanded: true });
    expect(updatedButton).toHaveAttribute("aria-expanded", "true");
  });

  it("collapses session card on second click", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const cardButton = screen.getByRole("button", { expanded: false });
    fireEvent.click(cardButton);
    expect(expandState["session-1"]).toBe(true);
    fireEvent.click(cardButton);
    expect(expandState["session-1"]).toBe(false);
  });

  it("renders error banner when history API returns 500", async () => {
    mockHook({
      error: "Unable to connect to the API server. Is the backend running?",
    });
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText(/Unable to connect/i)).toBeInTheDocument();
  });

  it("shows loading spinner when loading hook returns true", async () => {
    mockHook({ loading: true });
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("displays date picker input", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const dateInput = screen.getByTestId("date-picker");
    expect(dateInput).toBeInTheDocument();
    expect((dateInput as HTMLInputElement).value).toBe("2024-01-15");
  });

  it("calls setSelectedDate when date picker changes", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const dateInput = screen.getByTestId("date-picker");
    fireEvent.change(dateInput, { target: { value: "2025-06-01" } });
    // Verify the hook's setSelectedDate was called
    const hookReturn = (useHistory as any).mock.results[0].value;
    expect(hookReturn.setSelectedDate).toHaveBeenCalledWith("2025-06-01");
  });

  it("shows empty state when selected date has no sessions", async () => {
    mockHook({ sessions: [] });
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.getByText("No charge sessions yet")).toBeInTheDocument();
  });

  it("displays Load More button when sessions exist", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const loadMoreBtn = screen.getByText("Load More");
    expect(loadMoreBtn).toBeInTheDocument();
  });

  it("calls loadMore when button is clicked", async () => {
    const { loadMore } = mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const loadMoreBtn = screen.getByText("Load More");
    fireEvent.click(loadMoreBtn);
    expect(loadMore).toHaveBeenCalled();
  });

  it("Load More button is disabled while loading", async () => {
    mockHook({ loading: true });
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    // Loading state shows spinner, not Load More button
    expect(screen.queryByText("Load More")).not.toBeInTheDocument();
  });

  it("does not show Load More when no sessions", async () => {
    mockHook({ sessions: [] });
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(screen.queryByText("Load More")).not.toBeInTheDocument();
  });

  it("shows delete button for completed sessions", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const deleteBtn = screen.getByRole("button", {
      name: /Delete completed session/,
    });
    expect(deleteBtn).toBeInTheDocument();
  });

  it("opens ConfirmDialog when delete button is clicked", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const deleteBtn = screen.getByRole("button", {
      name: /Delete completed session/,
    });
    fireEvent.click(deleteBtn);
    expect(screen.getByText("Delete Session")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Delete this completed charge session? This cannot be undone.",
      ),
    ).toBeInTheDocument();
  });

  it("calls deleteSession when delete is confirmed", async () => {
    const { deleteSession } = mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const deleteBtn = screen.getByRole("button", {
      name: /Delete completed session/,
    });
    fireEvent.click(deleteBtn);
    fireEvent.click(screen.getByText("Delete"));
    await waitFor(() => {
      expect(deleteSession).toHaveBeenCalledWith("session-1");
    });
  });

  it("closes ConfirmDialog when cancel is clicked", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const deleteBtn = screen.getByRole("button", {
      name: /Delete completed session/,
    });
    fireEvent.click(deleteBtn);
    fireEvent.click(screen.getByText("Cancel"));
    expect(screen.queryByText("Delete Session")).not.toBeInTheDocument();
  });

  it("triggers delete on button activation", async () => {
    mockHook();
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    const deleteBtn = screen.getByRole("button", {
      name: /Delete completed session/,
    });
    fireEvent.click(deleteBtn);
    expect(screen.getByText("Delete Session")).toBeInTheDocument();
  });

  it("does not show delete button for active sessions", async () => {
    mockHook({
      sessions: [
        {
          id: "active-1",
          vehicleId: "vehicle-1",
          createdAt: "2024-01-15T10:00:00Z",
          startKwh: 0.76,
          startPercent: 20,
          targetPercent: 100,
          status: "active",
        },
      ],
    });
    customRender(<HistoryClient initialVehicles={[]} initialSessions={[]} />);
    expect(
      screen.queryByRole("button", { name: /Delete active session/ }),
    ).not.toBeInTheDocument();
  });
});
