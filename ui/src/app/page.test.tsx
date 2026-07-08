import { useVehicle, useChargeSession } from "@/hooks";
import { usePlug } from "@/hooks/usePlug";
import { useGaugeStore } from "@/stores/gaugeStore";
import { customRender, screen, fireEvent, waitFor } from "@/test-utils";
import { createPlug, createVehicle } from "@/test/fixtures";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

import Dashboard from "./Dashboard";

// Mock usePlug so Dashboard doesn't make real API calls for plugs.
// The new hook is vehicle-centric: selectedVehicleId/selectVehicle replaces selectedPlugId/selectPlug.
vi.mock("@/hooks/usePlug", () => ({
  usePlug: vi.fn(() => ({
    plugs: [
      createPlug({
        id: "default-plug",
        name: "Default Plug",
        online: false,
        namespace: "ns",
        mqttTopic: "t",
        tls: false,
        userId: "u",
        createdAt: "",
        vehicleId: "rm1",
        type: "charging",
      }),
    ],
    selectedVehicleId: "rm1",
    selectVehicle: vi.fn(),
    isLoading: false,
    error: null,
    createPlug: vi.fn(),
    isCreating: false,
    updatePlug: vi.fn(),
    deletePlug: vi.fn(),
    toggleMaintenancePower: vi.fn(),
    isTogglingPower: false,
  })),
}));

// Hoisted reference so the SpeedometerGauge mock can access the real store at runtime
const gaugeStoreRef = vi.hoisted(() => ({
  current: null as typeof useGaugeStore | null,
}));
gaugeStoreRef.current = useGaugeStore;

const defaultVehicles = [
  createVehicle({ id: "rm1", name: "Maeving RM1", capacityKwh: 1.9 }),
  createVehicle({ id: "rm2", name: "Maeving RM2", capacityKwh: 5.46 }),
];

// Mock the SettingsModal component
vi.mock("@/components/SettingsModal", () => ({
  default: ({ isOpen, onClose }: any) =>
    isOpen ? (
      <div data-testid="settings-modal">
        <button onClick={onClose}>Close</button>
      </div>
    ) : null,
}));

// Mock the PowerChart component
vi.mock("@/components/PowerChart", () => ({
  default: ({ vehicleId }: { vehicleId?: string }) => (
    <div data-testid="power-chart" data-vehicle-id={vehicleId}>
      <p>No active charge session</p>
    </div>
  ),
}));

// Mock the SpeedometerGauge component - stores callbacks for test access
const gaugeCallbacks: Record<string, (...args: any[]) => any> = {};
vi.mock("@/components/SpeedometerGauge", () => ({
  default: ({
    status,
    onStartStop,
    onDragEnd,
    powerDraw,
    errorMessage,
  }: any) => {
    const store = gaugeStoreRef.current;
    if (!store) return null;
    const { currentPercent, targetPercent } = store.getState();
    gaugeCallbacks.onStartStop = onStartStop;
    gaugeCallbacks.onDragEnd = onDragEnd;
    const statusText =
      status === "charging"
        ? "CHARGING"
        : status === "error"
          ? "ERROR"
          : "READY";
    return (
      <div data-testid="speedometer-gauge">
        <canvas aria-label="Speedometer gauge" />
        <div data-testid="current-percent">{currentPercent}%</div>
        <div data-testid="status">{statusText}</div>
        {errorMessage && <div data-testid="error-message">{errorMessage}</div>}
        <button
          onClick={onStartStop}
          disabled={status !== "charging" && currentPercent >= targetPercent}
          aria-label={
            status === "charging"
              ? "STOP"
              : currentPercent >= targetPercent
                ? "CHARGED"
                : "START"
          }
        >
          {status === "charging"
            ? "STOP"
            : currentPercent >= targetPercent
              ? "CHARGED"
              : "START"}
        </button>
        {powerDraw && <div data-testid="power-draw">{powerDraw}W</div>}
      </div>
    );
  },
}));

// Mock localStorage
const mockLocalStorage = {
  getItem: vi.fn<() => string | null>(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
};
Object.defineProperty(window, "localStorage", { value: mockLocalStorage });

// Mock hooks - factory creates vi.fn() mocks that tests can override via mockImplementation
vi.mock("@/hooks", () => ({
  useVehicle: vi.fn(() => ({
    vehicles: [
      {
        id: "rm1",
        name: "Maeving RM1",
        capacityKwh: 1.9,
        chargerOutputW: 600,
        chargingEfficiency: 0.8,
      },
      {
        id: "rm2",
        name: "Maeving RM2",
        capacityKwh: 5.46,
        chargerOutputW: 600,
        chargingEfficiency: 0.8,
      },
    ],
    handleOpenSettings: vi.fn(),
    isSettingsOpen: false,
    closeSettings: vi.fn(),
    isLoading: false,
    tempError: null,
    setTempError: vi.fn(),
    updatePercents: vi.fn(),
    updateNotificationPrefs: vi.fn(),
  })),
  useChargeSession: vi.fn(() => ({
    session: { status: "idle", powerDraw: 0 },
    chargeStartPercent: null,
    errorMessage: null,
    startCharging: vi.fn(),
    stopCharging: vi.fn(),
    isChargingActionPending: false,
    isStopActionPending: false,
    handleTargetChargeUpdate: vi.fn(),
    onDragStart: vi.fn(),
    onDragEnd: vi.fn(),
    clearError: vi.fn(),
    sessionStartTime: null,
  })),
  useHistory: vi.fn(() => ({
    sessions: [],
    vehicles: [],
    selectedVehicleId: null,
    loading: false,
    error: null,
    handleVehicleChange: vi.fn(),
    toggleExpand: vi.fn(),
    getVehicleName: vi.fn((id: string) => id),
    isExpanded: vi.fn(),
  })),
  useSchedule: vi.fn(() => ({
    schedule: null,
    isLoading: false,
    saveSchedule: vi.fn(),
  })),
  useCarbonIntensity: vi.fn(() => ({
    carbonIntensity: null,
  })),
}));

describe("Home Page", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockLocalStorage.getItem.mockImplementation(() => null);
    (usePlug as any).mockImplementation(() => ({
      plugs: [
        createPlug({
          id: "default-plug",
          name: "Default Plug",
          online: false,
          namespace: "ns",
          mqttTopic: "t",
          tls: false,
          userId: "u",
          createdAt: "",
        }),
      ],
      selectedVehicleId: null,
      selectVehicle: vi.fn(),
      toggleMaintenancePower: vi.fn(),
      isTogglingPower: false,
      isLoading: false,
      error: null,
      createPlug: vi.fn(),
      isCreating: false,
      updatePlug: vi.fn(),
      deletePlug: vi.fn(),
    }));
    (useVehicle as any).mockImplementation(() => ({
      vehicles: defaultVehicles,
      handleOpenSettings: vi.fn(),
      isSettingsOpen: false,
      closeSettings: vi.fn(),
      isLoading: false,
      tempError: null,
      setTempError: vi.fn(),
    }));
    (useChargeSession as any).mockImplementation(() => ({
      session: { status: "idle", powerDraw: 0 },
      chargeStartPercent: null,
      errorMessage: null,
      startCharging: vi.fn(),
      stopCharging: vi.fn(),
      isChargingActionPending: false,
      isStopActionPending: false,
      handleTargetChargeUpdate: vi.fn(),
      onDragStart: vi.fn(),
      onDragEnd: vi.fn(),
      clearError: vi.fn(),
      sessionStartTime: null,
    }));
  });

  function mockVehicle(
    opts: Partial<{
      vehicles: any[];
      isLoading: boolean;
      isSettingsOpen: boolean;
      updatePercents: any;
    }> = {},
  ) {
    (useVehicle as any).mockImplementation(() => ({
      vehicles: opts.vehicles ?? defaultVehicles,
      handleOpenSettings: vi.fn(),
      isSettingsOpen: opts.isSettingsOpen ?? false,
      closeSettings: vi.fn(),
      isLoading: opts.isLoading ?? false,
      tempError: null,
      setTempError: vi.fn(),
      updatePercents: opts.updatePercents ?? vi.fn().mockResolvedValue(true),
      updateNotificationPrefs: vi.fn(),
    }));
  }

  function mockChargeSession(
    opts: Partial<{
      session: any;
      errorMessage: string | null;
      startCharging: any;
      stopCharging: any;
      onDragEnd: any;
    }> = {},
  ) {
    (useChargeSession as any).mockImplementation(() => ({
      session: opts.session ?? { status: "idle", powerDraw: 0 },
      chargeStartPercent: null,
      errorMessage: opts.errorMessage ?? null,
      startCharging: opts.startCharging ?? vi.fn(),
      stopCharging: opts.stopCharging ?? vi.fn(),
      isChargingActionPending: false,
      isStopActionPending: false,
      handleTargetChargeUpdate: vi.fn(),
      onDragStart: vi.fn(),
      onDragEnd: opts.onDragEnd ?? vi.fn(),
      clearError: vi.fn(),
      sessionStartTime: null,
    }));
  }

  it("renders the page title", async () => {
    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByText(/EV Charge Controller/i)).toBeInTheDocument(),
    );
  });

  it("renders speedometer gauge", async () => {
    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByTestId("speedometer-gauge")).toBeInTheDocument(),
    );
  });

  it("displays vehicle info when selected", async () => {
    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByText(/EV Charge Controller/i)).toBeInTheDocument(),
    );
  });

  it("shows settings button", async () => {
    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByTitle(/settings/i)).toBeInTheDocument(),
    );
  });

  it("shows history link", async () => {
    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByTitle(/history/i)).toBeInTheDocument(),
    );
  });

  it("displays no vehicle message when not selected", async () => {
    customRender(<Dashboard />);
    await waitFor(() =>
      expect(
        screen.getByText(/no vehicle assigned to this plug/i),
      ).toBeInTheDocument(),
    );
  });

  it("displays current percent from gauge", async () => {
    customRender(<Dashboard />);
    await waitFor(() => expect(screen.getByText("20%")).toBeInTheDocument());
  });

  it("displays status from gauge", async () => {
    customRender(<Dashboard />);
    await waitFor(() => expect(screen.getByText(/READY/i)).toBeInTheDocument());
  });

  it("handles start charging button click", async () => {
    customRender(<Dashboard />);
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: /START/i }),
      ).toBeInTheDocument(),
    );
    const startButton = screen.getByRole("button", { name: /START/i });
    fireEvent.click(startButton);
    expect(startButton).toBeInTheDocument();
  });

  it("displays error message when start charging fails", async () => {
    mockVehicle({
      vehicles: defaultVehicles,
    });
    mockChargeSession({
      errorMessage:
        "The charge target must be higher than the current battery level. Adjust the target slider above the current level.",
    });

    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByTestId("error-message")).toBeInTheDocument(),
    );
  });

  it("shows CHARGED and disables button when current equals target", async () => {
    // Set vehicle percents to match (current == target) to trigger CHARGED state.
    // The Dashboard init effect reads from selectedVehicle, not the store directly.
    // Must also mock usePlug with a vehicleId so selectedVehicle is non-null.
    (usePlug as any).mockImplementation(() => ({
      plugs: [
        createPlug({
          id: "default-plug",
          name: "Default Plug",
          online: false,
          namespace: "ns",
          mqttTopic: "t",
          tls: false,
          userId: "u",
          createdAt: "",
        }),
      ],
      selectedVehicleId: "rm1",
      selectVehicle: vi.fn(),
      toggleMaintenancePower: vi.fn(),
      isTogglingPower: false,
      isLoading: false,
      error: null,
      createPlug: vi.fn(),
      isCreating: false,
      updatePlug: vi.fn(),
      deletePlug: vi.fn(),
    }));
    mockVehicle({
      vehicles: defaultVehicles.map((v) => ({
        ...v,
        currentPercent: 80,
        targetPercent: 80,
      })),
    });
    mockChargeSession({
      session: { status: "idle", powerDraw: 0 },
    });

    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByTestId("speedometer-gauge")).toBeInTheDocument(),
    );

    const startButton = screen.getByRole("button", { name: /CHARGED/i });
    expect(startButton).toBeInTheDocument();
    expect(startButton).toBeDisabled();
  });

  it("handles slider change", async () => {
    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByTestId("speedometer-gauge")).toBeInTheDocument(),
    );

    const canvas = screen
      .getByTestId("speedometer-gauge")
      .querySelector("canvas");
    fireEvent.click(canvas as Element, { clientX: 228, clientY: 160 });
    expect(canvas).toBeInTheDocument();
  });

  it("displays currentPercent from completed session", async () => {
    useGaugeStore.setState({
      currentPercent: 45,
      targetPercent: 90,
    });
    mockVehicle({
      vehicles: defaultVehicles,
    });
    mockChargeSession({
      session: { status: "idle", powerDraw: 0 },
    });

    customRender(<Dashboard />);

    await waitFor(() => {
      expect(screen.getByTestId("current-percent")).toBeInTheDocument();
    });
  });

  it("displays charging status when session is active", async () => {
    useGaugeStore.setState({
      currentPercent: 45,
      targetPercent: 90,
    });
    mockVehicle({
      vehicles: defaultVehicles,
    });
    mockChargeSession({
      session: { status: "charging", powerDraw: 300 },
    });

    customRender(<Dashboard />);
    await waitFor(() => {
      expect(screen.getByText(/CHARGING/i)).toBeInTheDocument();
    });
  });

  it("displays power draw when charging", async () => {
    useGaugeStore.setState({
      currentPercent: 45,
      targetPercent: 90,
    });
    mockVehicle({
      vehicles: defaultVehicles,
    });
    mockChargeSession({
      session: { status: "charging", powerDraw: 300 },
    });

    customRender(<Dashboard />);
    await waitFor(() => {
      expect(screen.getByTestId("power-draw")).toBeInTheDocument();
    });
  });

  it("handles API fetch errors gracefully", async () => {
    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByText(/EV Charge Controller/i)).toBeInTheDocument(),
    );
    await waitFor(() =>
      expect(
        screen.getByText(/no vehicle assigned to this plug/i),
      ).toBeInTheDocument(),
    );
  });

  it("renders SettingsModal when settings button is clicked", async () => {
    (useVehicle as any).mockImplementation(() => ({
      vehicles: defaultVehicles,
      handleOpenSettings: vi.fn(),
      isSettingsOpen: true,
      closeSettings: vi.fn(),
      isLoading: false,
      tempError: null,
      setTempError: vi.fn(),
    }));
    customRender(<Dashboard />);
    await waitFor(() => {
      expect(screen.getByTestId("settings-modal")).toBeInTheDocument();
    });
  });

  it("calls closeSettings when SettingsModal close button is clicked", async () => {
    const closeSettingsMock = vi.fn();
    (useVehicle as any).mockImplementation(() => ({
      vehicles: defaultVehicles,
      handleOpenSettings: vi.fn(),
      isSettingsOpen: true,
      closeSettings: closeSettingsMock,
      isLoading: false,
      tempError: null,
      setTempError: vi.fn(),
    }));
    customRender(<Dashboard />);
    await waitFor(() => {
      expect(screen.getByTestId("settings-modal")).toBeInTheDocument();
    });
    const closeButton = screen.getByRole("button", { name: /Close/i });
    fireEvent.click(closeButton);
    expect(closeSettingsMock).toHaveBeenCalled();
  });

  it("renders tempError banner when set", async () => {
    useGaugeStore.setState({
      currentPercent: 45,
      targetPercent: 90,
    });
    (useVehicle as any).mockImplementation(() => ({
      vehicles: defaultVehicles,
      handleOpenSettings: vi.fn(),
      isSettingsOpen: true,
      closeSettings: vi.fn(),
      isLoading: false,
      tempError: "Cannot change vehicle while a charge session is active",
      setTempError: vi.fn(),
    }));
    mockChargeSession({
      session: { status: "charging", powerDraw: 300 },
    });

    customRender(<Dashboard />);
    await waitFor(() => {
      expect(
        screen.getByText(
          /Cannot change vehicle while a charge session is active/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows loading spinner when vehicles API is slow", async () => {
    useGaugeStore.setState({
      currentPercent: 0,
      targetPercent: 80,
    });
    mockVehicle({
      vehicles: [],
      isLoading: true,
    });
    mockChargeSession({
      session: { status: "idle", powerDraw: 0 },
    });

    customRender(<Dashboard />);
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("calls stopCharging when STOP button is clicked", async () => {
    const stopChargingMock = vi.fn().mockResolvedValue(undefined);
    useGaugeStore.setState({
      currentPercent: 45,
      targetPercent: 90,
    });
    mockVehicle({
      vehicles: defaultVehicles,
    });
    mockChargeSession({
      session: { status: "charging", powerDraw: 300 },
      stopCharging: stopChargingMock,
    });

    customRender(<Dashboard />);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /STOP/i })).toBeInTheDocument();
    });
    const stopButton = screen.getByRole("button", { name: /STOP/i });
    fireEvent.click(stopButton);

    await waitFor(() => {
      expect(stopChargingMock).toHaveBeenCalled();
    });
  });

  it("handleDragEnd calls chargeOnDragEnd and updatePercents", async () => {
    const chargeOnDragEnd = vi.fn();
    const updatePercents = vi.fn().mockResolvedValue(true);
    useGaugeStore.setState({
      currentPercent: 30,
      targetPercent: 70,
    });
    // Plug must have vehicleId so Dashboard resolves selectedVehicle
    (usePlug as any).mockImplementation(() => ({
      plugs: [
        {
          id: "plug-1",
          name: "My Plug",
          online: true,
          vehicleId: "rm1",
          type: "charging",
          powerOn: false,
          namespace: "ns",
          mqttTopic: "t",
          tls: false,
          userId: "u",
          createdAt: "",
        },
      ],
      selectedVehicleId: "rm1",

      selectVehicle: vi.fn(),
      toggleMaintenancePower: vi.fn(),
      isTogglingPower: false,
      isLoading: false,
      error: null,
      createPlug: vi.fn(),
      isCreating: false,
      updatePlug: vi.fn(),
      deletePlug: vi.fn(),
    }));
    mockVehicle({
      updatePercents,
    });
    mockChargeSession({
      session: { status: "idle", powerDraw: 0 },
      onDragEnd: chargeOnDragEnd,
    });

    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByTestId("speedometer-gauge")).toBeInTheDocument(),
    );

    // Trigger handleDragEnd via the gauge's onDragEnd callback
    if (gaugeCallbacks.onDragEnd) {
      gaugeCallbacks.onDragEnd(50, 75);
    }

    expect(chargeOnDragEnd).toHaveBeenCalledWith(50, 75);
    // plugId is threaded through so onSettled can refresh the carbon-aware
    // schedule's forecast-based start estimate after the target percent changes.
    expect(updatePercents).toHaveBeenCalledWith("rm1", 50, 75, "plug-1");
  });

  it("handleDragEnd skips updatePercents without selected vehicle", async () => {
    const updatePercents = vi.fn();
    useGaugeStore.setState({
      currentPercent: 20,
      targetPercent: 80,
    });
    mockVehicle({
      vehicles: defaultVehicles,
      updatePercents,
    });
    mockChargeSession({
      session: { status: "idle", powerDraw: 0 },
    });

    customRender(<Dashboard />);
    await waitFor(() =>
      expect(screen.getByTestId("speedometer-gauge")).toBeInTheDocument(),
    );

    // Trigger handleDragEnd - should call chargeOnDragEnd but NOT updatePercents
    if (gaugeCallbacks.onDragEnd) {
      gaugeCallbacks.onDragEnd(30, 70);
    }

    expect(updatePercents).not.toHaveBeenCalled();
  });

  describe("Gauge percents on vehicle switch", () => {
    // Two vehicles with different percents - switching selectedVehicleId
    // must re-sync the gauge to the new vehicle's percents.
    const vehicleA = createVehicle({
      id: "vehicle-a",
      name: "Vehicle A",
      currentPercent: 10,
      targetPercent: 90,
    });
    const vehicleB = createVehicle({
      id: "vehicle-b",
      name: "Vehicle B",
      currentPercent: 40,
      targetPercent: 60,
    });
    const plugA = createPlug({
      id: "plug-a",
      name: "Plug A",
      vehicleId: "vehicle-a",
      type: "charging",
    });
    const plugB = createPlug({
      id: "plug-b",
      name: "Plug B",
      vehicleId: "vehicle-b",
      type: "charging",
    });

    let currentVehicleId = "vehicle-a";

    beforeEach(() => {
      currentVehicleId = "vehicle-a";
      useGaugeStore.setState({
        currentPercent: 20,
        targetPercent: 80,
        initialized: false,
      });
    });

    it("should re-sync gauge percents when switching vehicles", async () => {
      (usePlug as any).mockImplementation(() => ({
        plugs: [plugA, plugB],
        selectedVehicleId: currentVehicleId,
        selectVehicle: vi.fn(),
        toggleMaintenancePower: vi.fn(),
        isTogglingPower: false,
        isLoading: false,
        error: null,
        createPlug: vi.fn(),
        isCreating: false,
        updatePlug: vi.fn(),
        deletePlug: vi.fn(),
      }));
      (useVehicle as any).mockImplementation(() => ({
        vehicles: [vehicleA, vehicleB],
        handleOpenSettings: vi.fn(),
        isSettingsOpen: false,
        closeSettings: vi.fn(),
        isLoading: false,
        tempError: null,
        setTempError: vi.fn(),
        updatePercents: vi.fn(),
        updateNotificationPrefs: vi.fn(),
      }));

      const { rerender } = customRender(<Dashboard />);

      await waitFor(() => {
        expect(useGaugeStore.getState().initialized).toBe(true);
      });

      // Gauge should reflect Vehicle A's percents (10%/90%)
      expect(useGaugeStore.getState().currentPercent).toBe(10);
      expect(useGaugeStore.getState().targetPercent).toBe(90);

      // Simulate a drag that changes percents
      useGaugeStore.setState({ currentPercent: 50, targetPercent: 70 });

      // Switch to Vehicle B
      currentVehicleId = "vehicle-b";
      rerender(<Dashboard />);

      // Gauge must re-sync to Vehicle B's percents (40%/60%)
      await waitFor(
        () => {
          const state = useGaugeStore.getState();
          expect(state.currentPercent).toBe(40);
          expect(state.targetPercent).toBe(60);
        },
        { timeout: 3000 },
      );
    });
  });
});
