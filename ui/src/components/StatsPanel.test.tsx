import { useGaugeStore } from "@/stores/gaugeStore";
import { createVehicle } from "@/test/fixtures";
import { calculateETA } from "@/utils/eta";
import { render, screen, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import StatsPanel from "./StatsPanel";

const mockVehicle = createVehicle({
  chargerOutputW: 1200,
  rangeMinMi: 10,
  rangeMaxMi: 60,
  currentPercent: 25,
  targetPercent: 80,
});

const defaultProps = {
  status: "idle" as const,
  powerDraw: 0,
  energyAddedKwh: null,
  errorMessage: null,
  sessionStartTime: null,
  startPercent: 25,
  currentPercent: 25,
  targetPercent: 80,
  vehicle: mockVehicle,
  voltage: null,
  current: null,
  carbonIntensity: null,
};

beforeEach(() => {
  useGaugeStore.setState({ currentPercent: 25, targetPercent: 80 });
  vi.useFakeTimers();
});

describe("StatsPanel", () => {
  it("renders all stat cards", () => {
    render(<StatsPanel {...defaultProps} />);
    expect(screen.getByText("Power Draw")).toBeInTheDocument();
    expect(screen.getByText("Charge Duration")).toBeInTheDocument();
    expect(screen.getByText("Progress")).toBeInTheDocument();
    expect(screen.getByText("Time Remaining")).toBeInTheDocument();
    expect(screen.getByText("Energy Added")).toBeInTheDocument();
    expect(screen.getByText("Estimated Cost")).toBeInTheDocument();
    expect(screen.getByText("Actual Cost")).toBeInTheDocument();
    expect(screen.getByText("Current")).toBeInTheDocument();
    expect(screen.getByText("Voltage")).toBeInTheDocument();
    expect(screen.getByText("Energy Left")).toBeInTheDocument();
    expect(screen.getByText("Carbon Intensity")).toBeInTheDocument();
    expect(screen.getByText("CO₂ Saved")).toBeInTheDocument();
  });

  it("shows error message when provided", () => {
    render(
      <StatsPanel {...defaultProps} errorMessage="Something went wrong" />,
    );
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("shows progress as 0% when idle", () => {
    render(<StatsPanel {...defaultProps} />);
    expect(screen.getByText("0%")).toBeInTheDocument();
  });

  it("shows progress percentage when charging", () => {
    render(
      <StatsPanel
        {...defaultProps}
        status="charging"
        startPercent={25}
        currentPercent={50}
      />,
    );
    // (50-25)/(80-25) * 100 = 45.45... -> Math.floor -> 45%
    expect(screen.getByText("45%")).toBeInTheDocument();
  });

  it("uses Math.floor for progress percentage (not Math.round)", () => {
    render(
      <StatsPanel
        {...defaultProps}
        status="charging"
        startPercent={25}
        currentPercent={31}
      />,
    );
    // (31-25)/(80-25) * 100 = 10.909... -> Math.floor -> 10% (Math.round would give 11%)
    expect(screen.getByText("10%")).toBeInTheDocument();
  });

  it("shows progress capped at 100%", () => {
    render(
      <StatsPanel
        {...defaultProps}
        status="charging"
        startPercent={25}
        currentPercent={100}
      />,
    );
    expect(screen.getByText("100%")).toBeInTheDocument();
  });

  it("shows estimated cost when idle", () => {
    render(<StatsPanel {...defaultProps} />);
    const costEls = screen.getAllByText(/\£\d+\.\d{2}/);
    expect(costEls.length).toBeGreaterThanOrEqual(2);
  });

  it("shows actual cost when charging", () => {
    render(
      <StatsPanel {...defaultProps} status="charging" energyAddedKwh={0.5} />,
    );
    const costEls = screen.getAllByText(/\£\d+\.\d{2}/);
    expect(costEls.length).toBeGreaterThan(0);
  });

  it("shows range cards when vehicle has range data", () => {
    render(<StatsPanel {...defaultProps} />);
    expect(screen.getByText("Current Range")).toBeInTheDocument();
    expect(screen.getByText("Target Range")).toBeInTheDocument();
  });

  it("hides range cards when vehicle has no range data", () => {
    const vehicleNoRange = {
      ...mockVehicle,
      rangeMinMi: 0,
      rangeMaxMi: 0,
    };
    render(<StatsPanel {...defaultProps} vehicle={vehicleNoRange} />);
    expect(screen.queryByText("Current Range")).not.toBeInTheDocument();
    expect(screen.queryByText("Target Range")).not.toBeInTheDocument();
  });

  it("shows dash for time remaining when no ETA", () => {
    render(<StatsPanel {...defaultProps} vehicle={null} />);
    const dashes = screen.getAllByText("-");
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  it("shows elapsed duration when sessionStartTime is set", () => {
    const startTime = Date.now() - 120000; // 2 minutes ago
    render(
      <StatsPanel
        {...defaultProps}
        status="charging"
        sessionStartTime={startTime}
      />,
    );
    expect(screen.getByText("00:02:00")).toBeInTheDocument();
  });

  it("shows 00:00:00 duration when no session start time", () => {
    render(<StatsPanel {...defaultProps} />);
    expect(screen.getByText("00:00:00")).toBeInTheDocument();
  });

  it("updates elapsed duration over time", () => {
    const startTime = Date.now() - 60000;
    render(
      <StatsPanel
        {...defaultProps}
        status="charging"
        sessionStartTime={startTime}
      />,
    );
    expect(screen.getByText("00:01:00")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(60000);
    });
    expect(screen.getByText("00:02:00")).toBeInTheDocument();
  });

  it("shows power draw in watts", () => {
    render(<StatsPanel {...defaultProps} powerDraw={1500} />);
    expect(screen.getByText("1.50 kW")).toBeInTheDocument();
  });

  it("shows power draw in watts for low values", () => {
    render(<StatsPanel {...defaultProps} powerDraw={500} />);
    expect(screen.getByText("500 W")).toBeInTheDocument();
  });

  it("shows 0 W when no power draw", () => {
    render(<StatsPanel {...defaultProps} powerDraw={0} />);
    expect(screen.getByText("0 W")).toBeInTheDocument();
  });

  it("shows energy added with 2 decimal places", () => {
    render(<StatsPanel {...defaultProps} energyAddedKwh={1.234} />);
    expect(screen.getByText("1.23 kWh")).toBeInTheDocument();
  });

  it("shows 0.00 kWh when no energy added", () => {
    render(<StatsPanel {...defaultProps} energyAddedKwh={null} />);
    const els = screen.getAllByText("0.00 kWh");
    expect(els.length).toBeGreaterThanOrEqual(1);
  });

  it("uses default efficiency when vehicle is null", () => {
    render(<StatsPanel {...defaultProps} vehicle={null} />);
    // Should not crash
    expect(screen.getByText("Estimated Cost")).toBeInTheDocument();
  });

  it("clamps progress to 0% when current < start", () => {
    render(
      <StatsPanel
        {...defaultProps}
        status="charging"
        startPercent={25}
        currentPercent={20}
      />,
    );
    expect(screen.getByText("0%")).toBeInTheDocument();
  });

  it("shows target completion estimate when idle with vehicle", () => {
    render(<StatsPanel {...defaultProps} />);
    const targetCard = screen.getByText("Target Completion").closest("div");
    const targetValue = targetCard?.querySelector("div")?.textContent;
    expect(targetValue).not.toBe("-");
    expect(targetValue).toMatch(/\d{2}:\d{2}:\d{2}/);
  });

  it("shows time remaining estimate when idle with vehicle", () => {
    render(<StatsPanel {...defaultProps} />);
    const remainingCard = screen.getByText("Time Remaining").closest("div");
    const remainingValue = remainingCard?.querySelector("div")?.textContent;
    expect(remainingValue).not.toBe("-");
    expect(remainingValue).toMatch(/\d{2}:\d{2}:\d{2}/);
  });

  it("shows dash for target and remaining when no vehicle", () => {
    render(<StatsPanel {...defaultProps} vehicle={null} />);
    const dashes = screen.getAllByText("-");
    expect(dashes.length).toBeGreaterThanOrEqual(2);
  });

  it("duration shows 00:00:00 when idle", () => {
    render(<StatsPanel {...defaultProps} />);
    expect(screen.getByText("00:00:00")).toBeInTheDocument();
  });

  it("duration does not tick when idle", () => {
    render(<StatsPanel {...defaultProps} />);
    expect(screen.getByText("00:00:00")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(60000);
    });
    expect(screen.getByText("00:00:00")).toBeInTheDocument();
  });

  it("target completion ticks up when idle", () => {
    render(<StatsPanel {...defaultProps} />);
    const targetCard = screen.getByText("Target Completion").closest("div");
    const initialValue = targetCard?.querySelector("div")?.textContent;
    expect(initialValue).not.toBe("-");

    act(() => {
      vi.advanceTimersByTime(10000);
    });

    const updatedValue = targetCard?.querySelector("div")?.textContent;
    expect(updatedValue).not.toBe(initialValue);
    expect(updatedValue).toMatch(/\d{2}:\d{2}:\d{2}/);
  });

  it("time remaining stays constant when idle", () => {
    render(<StatsPanel {...defaultProps} />);
    const remainingCard = screen.getByText("Time Remaining").closest("div");
    const initialValue = remainingCard?.querySelector("div")?.textContent;
    expect(initialValue).not.toBe("-");

    act(() => {
      vi.advanceTimersByTime(30000);
    });

    const updatedValue = remainingCard?.querySelector("div")?.textContent;
    expect(updatedValue).toBe(initialValue);
  });

  it("shows estimate when transitioning from charging to idle", () => {
    const startTime = Date.now() - 120000;
    const { unmount } = render(
      <StatsPanel
        {...defaultProps}
        status="charging"
        sessionStartTime={startTime}
      />,
    );

    unmount();

    render(<StatsPanel {...defaultProps} />);
    const targetCard = screen.getByText("Target Completion").closest("div");
    const targetValue = targetCard?.querySelector("div")?.textContent;
    expect(targetValue).not.toBe("-");

    const remainingCard = screen.getByText("Time Remaining").closest("div");
    const remainingValue = remainingCard?.querySelector("div")?.textContent;
    expect(remainingValue).not.toBe("-");
  });

  it("eta updates when currentPercent changes while idle", () => {
    const { rerender } = render(<StatsPanel {...defaultProps} />);
    const initialRemaining = screen
      .getByText("Time Remaining")
      .closest("div")
      ?.querySelector("div")?.textContent;

    rerender(<StatsPanel {...defaultProps} currentPercent={50} />);
    const updatedRemaining = screen
      .getByText("Time Remaining")
      .closest("div")
      ?.querySelector("div")?.textContent;
    expect(updatedRemaining).not.toBe(initialRemaining);
  });

  it("eta updates when targetPercent changes while idle", () => {
    const { rerender } = render(<StatsPanel {...defaultProps} />);
    const initialRemaining = screen
      .getByText("Time Remaining")
      .closest("div")
      ?.querySelector("div")?.textContent;

    rerender(<StatsPanel {...defaultProps} targetPercent={100} />);
    const updatedRemaining = screen
      .getByText("Time Remaining")
      .closest("div")
      ?.querySelector("div")?.textContent;
    expect(updatedRemaining).not.toBe(initialRemaining);
  });

  it("shows dash for remaining when currentPercent >= targetPercent", () => {
    render(
      <StatsPanel {...defaultProps} currentPercent={90} targetPercent={80} />,
    );
    const dashes = screen.getAllByText("-");
    expect(dashes.length).toBeGreaterThanOrEqual(1);
  });

  it("carbon intensity shows value and unit", () => {
    render(
      <StatsPanel
        {...defaultProps}
        carbonIntensity={{ forecast: 150, actual: 150, index: "low" }}
      />,
    );
    expect(screen.getByText("150")).toBeInTheDocument();
    expect(screen.getByText("gCO₂/kWh")).toBeInTheDocument();
  });

  it("carbon intensity uses default when null", () => {
    render(<StatsPanel {...defaultProps} carbonIntensity={null} />);
    expect(screen.getByText("140")).toBeInTheDocument();
  });

  it("carbon intensity applies color class based on index", () => {
    const { rerender } = render(
      <StatsPanel
        {...defaultProps}
        carbonIntensity={{ forecast: 100, actual: 100, index: "very low" }}
      />,
    );
    const intensityValue = screen.getByText("100").closest("div");
    expect(intensityValue?.className).toContain("text-success");

    rerender(
      <StatsPanel
        {...defaultProps}
        carbonIntensity={{ forecast: 250, actual: 250, index: "very high" }}
      />,
    );
    const highIntensityValue = screen.getByText("250").closest("div");
    expect(highIntensityValue?.className).toContain("text-danger");
  });

  it("co2 saved shows estimate when idle", () => {
    render(
      <StatsPanel
        {...defaultProps}
        carbonIntensity={{ forecast: 200, actual: 200, index: "moderate" }}
      />,
    );
    const co2Card = screen.getByText("CO₂ Saved").closest("div");
    const co2Value = co2Card?.textContent;
    expect(co2Value).not.toBe("0 g");
  });

  it("co2 saved shows actual when charging with energy added", () => {
    render(
      <StatsPanel
        {...defaultProps}
        status="charging"
        currentPercent={50}
        energyAddedKwh={1.5}
        carbonIntensity={{ forecast: 200, actual: 200, index: "moderate" }}
      />,
    );
    const co2Card = screen.getByText("CO₂ Saved").closest("div");
    const co2Value = co2Card?.textContent;
    expect(co2Value).not.toBe("0 g");
  });

  it("co2 saved shows 0 g when no energy and no estimate possible", () => {
    render(
      <StatsPanel
        {...defaultProps}
        status="charging"
        currentPercent={90}
        targetPercent={80}
        energyAddedKwh={null}
        carbonIntensity={{ forecast: 200, actual: 200, index: "moderate" }}
      />,
    );
    expect(screen.getByText("0 g")).toBeInTheDocument();
  });

  it("energy left shows battery-side estimate when idle", () => {
    render(<StatsPanel {...defaultProps} />);
    const energyLeftCard = screen.getByText("Energy Left").closest("div");
    const energyValue = energyLeftCard?.textContent;
    expect(energyValue).toMatch(/\d+\.\d+ kWh/);
  });

  it("energy left shows 0.00 kWh when current >= target", () => {
    render(
      <StatsPanel {...defaultProps} currentPercent={90} targetPercent={80} />,
    );
    const energyLeftCard = screen.getByText("Energy Left").closest("div");
    expect(energyLeftCard?.textContent).toContain("0.00 kWh");
  });

  it("cost always formatted as £X.XX", () => {
    render(
      <StatsPanel {...defaultProps} status="charging" energyAddedKwh={0.5} />,
    );
    const costEls = screen.getAllByText(/\£\d+\.\d{2}/);
    expect(costEls.length).toBeGreaterThanOrEqual(2);
    costEls.forEach((el) => {
      expect(el.textContent).toMatch(/\£\d+\.\d{2}/);
    });
  });

  it("estimated cost is 0 when current >= target", () => {
    render(
      <StatsPanel {...defaultProps} currentPercent={90} targetPercent={80} />,
    );
    const estimatedCard = screen.getByText("Estimated Cost").closest("div");
    expect(estimatedCard?.textContent).toContain("£0.00");
  });

  it("actual cost is £0.00 when no energy added", () => {
    render(
      <StatsPanel {...defaultProps} status="charging" energyAddedKwh={null} />,
    );
    const actualCard = screen.getByText("Actual Cost").closest("div");
    expect(actualCard?.textContent).toContain("£0.00");
  });

  it("voltage and current show values when provided", () => {
    render(<StatsPanel {...defaultProps} voltage={230} current={6.5} />);
    expect(screen.getByText("230 V")).toBeInTheDocument();
    expect(screen.getByText("6.50 A")).toBeInTheDocument();
  });

  it("voltage and current show 0 when null", () => {
    render(<StatsPanel {...defaultProps} voltage={null} current={null} />);
    expect(screen.getByText("0 V")).toBeInTheDocument();
    expect(screen.getByText("0.00 A")).toBeInTheDocument();
  });
});

describe("StatsPanel timing synchronization", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  function getStatValue(label: string): string {
    const card = screen.getByText(label).closest("div");
    return card?.querySelector("div.text-2xl")?.textContent ?? "";
  }

  function parseHMS(hms: string): number {
    const parts = hms.split(":").map(Number);
    return (parts[0] ?? 0) * 3600 + (parts[1] ?? 0) * 60 + (parts[2] ?? 0);
  }

  function getDurationSec(): number {
    return parseHMS(getStatValue("Charge Duration"));
  }

  function getRemainingSec(): number {
    return parseHMS(getStatValue("Time Remaining"));
  }

  // Compute expected total seconds for the mock vehicle from 25%→80%.
  // Used as the invariant that duration + remaining must always equal.
  function etaSec(from: number, to: number): number {
    const eta =
      calculateETA({
        currentPercent: from,
        targetPercent: to,
        capacityKwh: mockVehicle.capacityKwh,
        chargerOutputW: mockVehicle.chargerOutputW,
        chargingEfficiency: mockVehicle.chargingEfficiency,
        time0to80Min: null,
        time0to100Min: null,
        time20to80Min: null,
        time20to100Min: null,
      }) ?? 0;
    return Math.round(eta * 60);
  }

  const chargingProps = { ...defaultProps, status: "charging" as const };

  beforeEach(() => {
    useGaugeStore.setState({ currentPercent: 25, targetPercent: 80 });
    vi.useFakeTimers();
  });

  it("duration + remaining equals initial ETA at session start", () => {
    const startTime = Date.now();
    render(<StatsPanel {...chargingProps} sessionStartTime={startTime} />);
    expect(getDurationSec() + getRemainingSec()).toBe(etaSec(25, 80));
  });

  it("duration ticks up and remaining ticks down by the same amount", () => {
    const startTime = Date.now();
    render(<StatsPanel {...chargingProps} sessionStartTime={startTime} />);

    act(() => {
      vi.advanceTimersByTime(30000);
    });
    const dur1 = getDurationSec();
    const rem1 = getRemainingSec();

    act(() => {
      vi.advanceTimersByTime(30000);
    });
    const dur2 = getDurationSec();
    const rem2 = getRemainingSec();

    expect(dur2 - dur1).toBe(30);
    expect(rem1 - rem2).toBe(30);
  });

  it("duration + remaining sum stays constant as timer advances", () => {
    const startTime = Date.now();
    render(<StatsPanel {...chargingProps} sessionStartTime={startTime} />);
    const total = etaSec(25, 80);

    act(() => {
      vi.advanceTimersByTime(60000);
    });
    expect(getDurationSec() + getRemainingSec()).toBe(total);

    act(() => {
      vi.advanceTimersByTime(60000);
    });
    expect(getDurationSec() + getRemainingSec()).toBe(total);
  });

  it("SOC update mid-charge does not change duration + remaining sum", () => {
    // Regression: previously each SOC poll recalculated totalTimeMin via
    // Math.ceil(ETA) + elapsed, causing cumulative drift of several seconds.
    const startTime = Date.now();
    const { rerender } = render(
      <StatsPanel
        {...chargingProps}
        sessionStartTime={startTime}
        currentPercent={25}
      />,
    );

    act(() => {
      vi.advanceTimersByTime(30000);
    });
    const sumBefore = getDurationSec() + getRemainingSec();

    rerender(
      <StatsPanel
        {...chargingProps}
        sessionStartTime={startTime}
        currentPercent={30}
      />,
    );
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(getDurationSec() + getRemainingSec()).toBe(sumBefore);
  });

  it("multiple consecutive SOC updates do not cause drift", () => {
    // Regression: 10 SOC ticks would previously add up to ~6s of drift.
    const startTime = Date.now();
    const { rerender } = render(
      <StatsPanel
        {...chargingProps}
        sessionStartTime={startTime}
        currentPercent={25}
      />,
    );
    const total = etaSec(25, 80);

    for (let pct = 26; pct <= 35; pct++) {
      act(() => {
        vi.advanceTimersByTime(30000);
      });
      rerender(
        <StatsPanel
          {...chargingProps}
          sessionStartTime={startTime}
          currentPercent={pct}
        />,
      );
    }
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(getDurationSec() + getRemainingSec()).toBe(total);
  });

  it("sum is exact with no floor-rounding error", () => {
    // Regression: previously both formatDuration and formatEstimatedTime called
    // Math.floor independently, so the displayed sum could be totalTimeSec - 1.
    const startTime = Date.now();
    render(<StatsPanel {...chargingProps} sessionStartTime={startTime} />);

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(getDurationSec()).toBe(1);
    expect(getRemainingSec()).toBe(etaSec(25, 80) - 1);
    expect(getDurationSec() + getRemainingSec()).toBe(etaSec(25, 80));
  });

  it("target change mid-charge anchors sum to new ETA plus elapsed at change", () => {
    const startTime = Date.now();
    const { rerender } = render(
      <StatsPanel
        {...chargingProps}
        sessionStartTime={startTime}
        targetPercent={80}
      />,
    );

    act(() => {
      vi.advanceTimersByTime(60000);
    });
    expect(getDurationSec()).toBe(60);

    rerender(
      <StatsPanel
        {...chargingProps}
        sessionStartTime={startTime}
        targetPercent={90}
      />,
    );
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    // New total = ETA(25→90) + 60s elapsed at the moment of target change
    const expectedSum = etaSec(25, 90) + 60;
    expect(getDurationSec()).toBe(61);
    expect(getDurationSec() + getRemainingSec()).toBe(expectedSum);
  });
});
