import { useGaugeStore } from "@/stores/gaugeStore";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { GaugeOverlay } from "./GaugeOverlay";

beforeEach(() => {
  useGaugeStore.setState({ currentPercent: 50, targetPercent: 80 });
});

afterEach(() => {
  useGaugeStore.setState({
    currentPercent: 20,
    targetPercent: 80,
    isDragging: "none",
  });
});

describe("GaugeOverlay", () => {
  it("displays current percent from store", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(screen.getByText("50%")).toBeInTheDocument();
  });

  it("displays Ready status when idle", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(screen.getByText("Ready")).toBeInTheDocument();
  });

  it("displays Charging status when charging", () => {
    render(
      <GaugeOverlay
        status="charging"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(screen.getByText("Charging")).toBeInTheDocument();
  });

  it("displays Holding status when holding", () => {
    render(
      <GaugeOverlay
        status="holding"
        currentPercent={64}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(screen.getByText("Holding")).toBeInTheDocument();
  });

  it("shows estimated resume time when holding with an estimate", () => {
    render(
      <GaugeOverlay
        status="holding"
        currentPercent={64}
        targetPercent={80}
        onStartStop={() => {}}
        estimatedResumeTime="23:30"
      />,
    );
    expect(screen.getByTestId("estimated-resume-time")).toHaveTextContent(
      "23:30",
    );
  });

  it("does not show estimated resume time when holding without an estimate", () => {
    render(
      <GaugeOverlay
        status="holding"
        currentPercent={64}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(
      screen.queryByTestId("estimated-resume-time"),
    ).not.toBeInTheDocument();
  });

  it("does not show estimated resume time when charging even with an estimate present", () => {
    render(
      <GaugeOverlay
        status="charging"
        currentPercent={64}
        targetPercent={80}
        onStartStop={() => {}}
        estimatedResumeTime="23:30"
      />,
    );
    expect(
      screen.queryByTestId("estimated-resume-time"),
    ).not.toBeInTheDocument();
  });

  it("shows STOP button when holding", () => {
    render(
      <GaugeOverlay
        status="holding"
        currentPercent={64}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Stop charging" }),
    ).toHaveTextContent("STOP");
  });

  it("shows START button when idle", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Start charging" }),
    ).toHaveTextContent("START");
  });

  it("shows STOP button when charging", () => {
    render(
      <GaugeOverlay
        status="charging"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Stop charging" }),
    ).toHaveTextContent("STOP");
  });

  it("calls onStartStop when button clicked", () => {
    const onStartStop = vi.fn();
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={onStartStop}
      />,
    );
    screen.getByRole("button", { name: "Start charging" }).click();
    expect(onStartStop).toHaveBeenCalled();
  });

  it("disables button when charged", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={80}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: "Charged" })).toBeDisabled();
  });

  it("disables button when disconnected", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
        tasmotaConnected={false}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Start charging" }),
    ).toBeDisabled();
  });

  it("does not render 12V circle when maintenance is null", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
      />,
    );
    expect(screen.queryByTestId("maintenance-circle")).not.toBeInTheDocument();
  });

  it("renders 12V circle when maintenance plug is present and on", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
        maintenance={{ powerOn: true, online: true }}
        onToggleMaintenance={() => {}}
      />,
    );
    const btn = screen.getByTestId("maintenance-circle");
    expect(btn).toBeInTheDocument();
    expect(btn).toHaveAttribute("aria-checked", "true");
    expect(btn).toHaveAttribute(
      "aria-label",
      "12V charger on - tap to turn off",
    );
  });

  it("renders 12V circle with offline label when offline", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
        maintenance={{ powerOn: false, online: false }}
        onToggleMaintenance={() => {}}
      />,
    );
    const btn = screen.getByTestId("maintenance-circle");
    expect(btn).toHaveAttribute("aria-label", "12V charger offline");
  });

  it("calls onToggleMaintenance when 12V circle clicked", () => {
    const onToggle = vi.fn();
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
        maintenance={{ powerOn: false, online: true }}
        onToggleMaintenance={onToggle}
      />,
    );
    screen.getByTestId("maintenance-circle").click();
    expect(onToggle).toHaveBeenCalled();
  });

  it("disables 12V circle when maintenance toggle is pending", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
        maintenance={{ powerOn: true, online: true }}
        onToggleMaintenance={() => {}}
        isMaintenancePending={true}
      />,
    );
    expect(screen.getByTestId("maintenance-circle")).toBeDisabled();
  });

  it("shows the forecast-based estimated start time for a carbon-aware schedule", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
        schedule={{
          id: "s1",
          type: "carbon_aware",
          time: "22:00",
          windowStart: "22:00",
          windowEnd: "06:00",
          estimatedStartTime: "23:30",
          enabled: true,
        }}
      />,
    );
    expect(screen.getByText("23:30")).toBeInTheDocument();
    expect(screen.queryByText("06:00")).not.toBeInTheDocument();
    expect(screen.getByTestId("schedule-circle")).toHaveAttribute(
      "aria-label",
      "Schedule active - starts at 23:30",
    );
  });

  it("falls back to the ready-by time when no estimate is available yet", () => {
    render(
      <GaugeOverlay
        status="idle"
        currentPercent={50}
        targetPercent={80}
        onStartStop={() => {}}
        schedule={{
          id: "s1",
          type: "carbon_aware",
          time: "22:00",
          windowStart: "22:00",
          windowEnd: "06:00",
          enabled: true,
        }}
      />,
    );
    expect(screen.getByText("06:00")).toBeInTheDocument();
    expect(screen.getByTestId("schedule-circle")).toHaveAttribute(
      "aria-label",
      "Schedule active - ready by 06:00",
    );
  });
});
