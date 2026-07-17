import { useGaugeStore } from "@/stores/gaugeStore";
import { createPlug } from "@/test/fixtures";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ChargeControlProps } from "./ChargeControl";

import ChargeControl from "./ChargeControl";

const testVehicle = {
  id: "v1",
  name: "Test Vehicle",
  capacityKwh: 5.46,
  chargerOutputW: 600,
  chargingEfficiency: 0.8,
  rangeMinMi: 0,
  rangeMaxMi: 0,
};

vi.mock("@/components/SpeedometerGauge", () => ({
  default: ({ status, onStartStop, onDragEnd }: any) => {
    const store = useGaugeStore.getState();
    return (
      <div data-testid="speedometer-gauge">
        <div data-testid="gauge-status">{status}</div>
        <div data-testid="gauge-current">{store.currentPercent}%</div>
        <button onClick={onStartStop} data-testid="start-stop-button" />
        <button
          onClick={() => onDragEnd(store.currentPercent, store.targetPercent)}
          data-testid="drag-end-button"
        />
      </div>
    );
  },
}));

vi.mock("@/components/StatsPanel", () => ({
  default: ({ powerDraw, errorMessage }: any) => (
    <div data-testid="stats-panel">
      {errorMessage && <div data-testid="error-message">{errorMessage}</div>}
      {powerDraw > 0 && <div data-testid="power-draw">{powerDraw}W</div>}
    </div>
  ),
}));

vi.mock("@/components/ErrorBoundary", () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="error-boundary">{children}</div>
  ),
}));

const defaultProps: ChargeControlProps = {
  selectedVehicle: testVehicle,
  gauge: {
    currentPercent: 20,
    targetPercent: 80,
    startPercent: 20,
    status: "idle",
  },
  session: {
    isChargingOrPending: false,
    isActionPending: false,
    sessionStartTime: null,
  },
  telemetry: {
    powerDraw: 0,
    energyAddedKwh: null,
    voltage: null,
    current: null,
  },
  errorMessage: null,
  tasmotaConnected: true,
  carbonIntensity: null,
  isActive: false,
  handlers: {
    onStartCharging: vi.fn(),
    onStopCharging: vi.fn(),
    onDragStart: vi.fn(),
    onChargeDragEnd: vi.fn(),
    clearError: vi.fn(),
    handleTargetChargeUpdate: vi.fn(),
    updatePercents: vi.fn(),
  },
};

function renderComponent(props: Partial<ChargeControlProps> = {}) {
  return render(<ChargeControl {...defaultProps} {...props} />);
}

beforeEach(() => {
  vi.clearAllMocks();
  useGaugeStore.setState({
    currentPercent: 20,
    targetPercent: 80,
    isDragging: "none",
    initialized: true,
  });
});

describe("ChargeControl", () => {
  it("renders SpeedometerGauge", () => {
    renderComponent();
    expect(screen.getByTestId("speedometer-gauge")).toBeInTheDocument();
  });

  it("passes session status to gauge", () => {
    renderComponent();
    expect(screen.getByTestId("gauge-status")).toHaveTextContent("idle");
  });

  it("calls onStartCharging when start button clicked and not charging", () => {
    const onStartCharging = vi.fn();
    renderComponent({
      handlers: { ...defaultProps.handlers, onStartCharging },
    });
    fireEvent.click(screen.getByTestId("start-stop-button"));
    expect(onStartCharging).toHaveBeenCalledTimes(1);
    expect(defaultProps.handlers.onStopCharging).not.toHaveBeenCalled();
  });

  it("calls onStopCharging when start button clicked and charging", () => {
    const onStopCharging = vi.fn();
    renderComponent({
      gauge: {
        currentPercent: 20,
        targetPercent: 80,
        startPercent: 20,
        status: "charging",
      },
      session: {
        isChargingOrPending: true,
        isActionPending: false,
        sessionStartTime: null,
      },
      handlers: { ...defaultProps.handlers, onStopCharging },
    });
    fireEvent.click(screen.getByTestId("start-stop-button"));
    expect(onStopCharging).toHaveBeenCalledTimes(1);
    expect(defaultProps.handlers.onStartCharging).not.toHaveBeenCalled();
  });

  it("calls onStopCharging when start button clicked and pending", () => {
    const onStopCharging = vi.fn();
    renderComponent({
      gauge: {
        currentPercent: 20,
        targetPercent: 80,
        startPercent: 20,
        status: "pending",
      },
      session: {
        isChargingOrPending: true,
        isActionPending: false,
        sessionStartTime: null,
      },
      handlers: { ...defaultProps.handlers, onStopCharging },
    });
    fireEvent.click(screen.getByTestId("start-stop-button"));
    expect(onStopCharging).toHaveBeenCalledTimes(1);
  });

  describe("commit on drag end (single path)", () => {
    it("idle + vehicle: persists via updatePercents exactly once", () => {
      const onChargeDragEnd = vi.fn();
      const updatePercents = vi.fn();
      const handleTargetChargeUpdate = vi.fn();
      const clearError = vi.fn();
      useGaugeStore.setState({ currentPercent: 35, targetPercent: 90 });
      renderComponent({
        handlers: {
          ...defaultProps.handlers,
          onChargeDragEnd,
          updatePercents,
          handleTargetChargeUpdate,
          clearError,
        },
      });
      fireEvent.click(screen.getByTestId("drag-end-button"));
      expect(onChargeDragEnd).toHaveBeenCalledWith(35, 90);
      expect(clearError).toHaveBeenCalledTimes(1);
      expect(updatePercents).toHaveBeenCalledTimes(1);
      expect(updatePercents).toHaveBeenCalledWith("v1", 35, 90);
      expect(handleTargetChargeUpdate).not.toHaveBeenCalled();
    });

    it("idle + no vehicle: clears + ends drag but does not persist", () => {
      const onChargeDragEnd = vi.fn();
      const updatePercents = vi.fn();
      renderComponent({
        selectedVehicle: null,
        handlers: { ...defaultProps.handlers, onChargeDragEnd, updatePercents },
      });
      fireEvent.click(screen.getByTestId("drag-end-button"));
      expect(onChargeDragEnd).toHaveBeenCalledWith(20, 80);
      expect(updatePercents).not.toHaveBeenCalled();
    });

    it("active: routes to handleTargetChargeUpdate, never updatePercents", () => {
      const onChargeDragEnd = vi.fn();
      const updatePercents = vi.fn();
      const handleTargetChargeUpdate = vi.fn();
      useGaugeStore.setState({ currentPercent: 40, targetPercent: 95 });
      renderComponent({
        isActive: true,
        gauge: {
          currentPercent: 40,
          targetPercent: 95,
          startPercent: 20,
          status: "charging",
        },
        session: {
          isChargingOrPending: true,
          isActionPending: false,
          sessionStartTime: null,
        },
        handlers: {
          ...defaultProps.handlers,
          onChargeDragEnd,
          updatePercents,
          handleTargetChargeUpdate,
        },
      });
      fireEvent.click(screen.getByTestId("drag-end-button"));
      expect(onChargeDragEnd).toHaveBeenCalledWith(40, 95);
      expect(handleTargetChargeUpdate).toHaveBeenCalledTimes(1);
      expect(handleTargetChargeUpdate).toHaveBeenCalledWith(40, 95);
      expect(updatePercents).not.toHaveBeenCalled();
    });
  });

  describe("no live persistence during interaction (flood regression)", () => {
    it("idle: store changes alone never trigger a persist", async () => {
      const updatePercents = vi.fn();
      const handleTargetChargeUpdate = vi.fn();
      renderComponent({
        handlers: {
          ...defaultProps.handlers,
          updatePercents,
          handleTargetChargeUpdate,
        },
      });

      // Simulate the rapid intermediate updates produced by a drag.
      await act(async () => {
        for (let pct = 21; pct <= 50; pct++) {
          useGaugeStore.setState({ currentPercent: pct });
        }
      });

      expect(updatePercents).not.toHaveBeenCalled();
      expect(handleTargetChargeUpdate).not.toHaveBeenCalled();
    });

    it("charging: store changes alone never trigger a target update", async () => {
      const handleTargetChargeUpdate = vi.fn();
      renderComponent({
        isActive: true,
        gauge: {
          currentPercent: 40,
          targetPercent: 80,
          startPercent: 20,
          status: "charging",
        },
        session: {
          isChargingOrPending: true,
          isActionPending: false,
          sessionStartTime: null,
        },
        handlers: { ...defaultProps.handlers, handleTargetChargeUpdate },
      });

      await act(async () => {
        for (let pct = 81; pct <= 95; pct++) {
          useGaugeStore.setState({ targetPercent: pct });
        }
      });

      expect(handleTargetChargeUpdate).not.toHaveBeenCalled();
    });
  });

  describe("battery-less (generic) vehicle", () => {
    const genericVehicle = {
      ...testVehicle,
      name: "My Petrol Bike",
      capacityKwh: 0,
      chargerOutputW: 0,
      chargingEfficiency: 1,
    };

    it("renders the maintenance-only panel instead of the gauge", () => {
      renderComponent({
        selectedVehicle: genericVehicle,
        maintenancePlug: createPlug({
          id: "m1",
          name: "12V Charger",
          type: "maintenance",
          online: true,
          powerOn: true,
        }),
      });
      expect(screen.queryByTestId("speedometer-gauge")).not.toBeInTheDocument();
      expect(screen.queryByTestId("stats-panel")).not.toBeInTheDocument();
      expect(screen.getByText("12V Charger")).toBeInTheDocument();
    });

    it("prompts to add a 12V charger when the vehicle has none", () => {
      renderComponent({
        selectedVehicle: genericVehicle,
        maintenancePlug: null,
      });
      expect(
        screen.getByText(/No 12V maintenance charger configured/i),
      ).toBeInTheDocument();
    });
  });

  it("wraps SpeedometerGauge in ErrorBoundary", () => {
    renderComponent();
    expect(screen.getByTestId("error-boundary")).toBeInTheDocument();
  });

  it("passes errorMessage to gauge", () => {
    renderComponent({ errorMessage: "Test error" });
    expect(screen.getAllByTestId("error-message")[0]).toHaveTextContent(
      "Test error",
    );
  });

  it("passes powerDraw to gauge", () => {
    renderComponent({
      telemetry: { ...defaultProps.telemetry, powerDraw: 300 },
    });
    expect(screen.getAllByTestId("power-draw")[0]).toHaveTextContent("300W");
  });

  it("does not render power draw when zero", () => {
    renderComponent({ telemetry: { ...defaultProps.telemetry, powerDraw: 0 } });
    expect(screen.queryByTestId("power-draw")).not.toBeInTheDocument();
  });
});
