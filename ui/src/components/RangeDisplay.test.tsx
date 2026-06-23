import { useGaugeStore } from "@/stores/gaugeStore";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import RangeDisplay from "./RangeDisplay";

beforeEach(() => {
  useGaugeStore.setState({ currentPercent: 50, targetPercent: 80 });
});

describe("RangeDisplay", () => {
  it("renders min-max range with current and target", () => {
    render(<RangeDisplay rangeMinMi={29} rangeMaxMi={40} />);

    // min=29*50%=15, max=40*50%=20 for current; min=29*80%=23, max=40*80%=32 for target
    expect(screen.getByText("15-20 mi")).toBeInTheDocument();
    expect(screen.getByText("23-32 mi")).toBeInTheDocument();
  });

  it("renders single value when min and max are equal", () => {
    render(<RangeDisplay rangeMinMi={40} rangeMaxMi={40} />);

    expect(screen.getByText("20 mi")).toBeInTheDocument();
    expect(screen.getByText("32 mi")).toBeInTheDocument();
  });

  it("renders only current (red) when current and target match at 100%", () => {
    useGaugeStore.setState({ currentPercent: 100, targetPercent: 100 });
    render(<RangeDisplay rangeMinMi={29} rangeMaxMi={40} />);

    expect(screen.getByText("29-40 mi")).toBeInTheDocument();
    expect(screen.queryByText("→")).not.toBeInTheDocument();

    const el = screen.getByText("29-40 mi");
    expect(el).toHaveClass("text-red-400");
  });

  it("renders only current (red) when current and target match at same percent", () => {
    useGaugeStore.setState({ currentPercent: 50, targetPercent: 50 });
    render(<RangeDisplay rangeMinMi={29} rangeMaxMi={40} />);

    expect(screen.getByText("15-20 mi")).toBeInTheDocument();
    expect(screen.queryByText("→")).not.toBeInTheDocument();

    const el = screen.getByText("15-20 mi");
    expect(el).toHaveClass("text-red-400");
  });

  it("renders only target (orange) when current percent is 0", () => {
    useGaugeStore.setState({ currentPercent: 0, targetPercent: 80 });
    render(<RangeDisplay rangeMinMi={29} rangeMaxMi={40} />);

    expect(screen.getByText("23-32 mi")).toBeInTheDocument();
    expect(screen.queryByText("→")).not.toBeInTheDocument();

    const el = screen.getByText("23-32 mi");
    expect(el).toHaveClass("text-orange-400");
  });

  it("renders current (red) → target (orange) when ranges differ", () => {
    useGaugeStore.setState({ currentPercent: 50, targetPercent: 80 });
    render(<RangeDisplay rangeMinMi={29} rangeMaxMi={40} />);

    expect(screen.getByText("15-20 mi")).toBeInTheDocument();
    expect(screen.getByText("23-32 mi")).toBeInTheDocument();
    expect(screen.getByText("→")).toBeInTheDocument();

    const currentEl = screen.getByText("15-20 mi");
    expect(currentEl).toHaveClass("text-red-400");

    const targetEl = screen.getByText("23-32 mi");
    expect(targetEl).toHaveClass("text-orange-400");
  });

  it("renders current (red) → 0 mi (orange) when target is 0", () => {
    useGaugeStore.setState({ currentPercent: 50, targetPercent: 0 });
    render(<RangeDisplay rangeMinMi={29} rangeMaxMi={40} />);

    expect(screen.getByText("15-20 mi")).toBeInTheDocument();
    expect(screen.getByText("0 mi")).toBeInTheDocument();
    expect(screen.getByText("→")).toBeInTheDocument();

    const currentEl = screen.getByText("15-20 mi");
    expect(currentEl).toHaveClass("text-red-400");

    const targetEl = screen.getByText("0 mi");
    expect(targetEl).toHaveClass("text-orange-400");
  });

  it("renders single value correctly with current → target when min equals max", () => {
    useGaugeStore.setState({ currentPercent: 50, targetPercent: 80 });
    render(<RangeDisplay rangeMinMi={115} rangeMaxMi={115} />);

    expect(screen.getByText("58 mi")).toBeInTheDocument();
    expect(screen.getByText("92 mi")).toBeInTheDocument();
    expect(screen.getByText("→")).toBeInTheDocument();
  });

  it("renders only current (red) when ranges match with equal min/max", () => {
    useGaugeStore.setState({ currentPercent: 60, targetPercent: 60 });
    render(<RangeDisplay rangeMinMi={100} rangeMaxMi={100} />);

    expect(screen.getByText("60 mi")).toBeInTheDocument();
    expect(screen.queryByText("→")).not.toBeInTheDocument();

    const el = screen.getByText("60 mi");
    expect(el).toHaveClass("text-red-400");
  });

  it("renders nothing when range is 0", () => {
    render(<RangeDisplay rangeMinMi={0} rangeMaxMi={0} />);

    expect(screen.queryByText("mi")).not.toBeInTheDocument();
  });

  it("rounds range values correctly", () => {
    useGaugeStore.setState({ currentPercent: 33, targetPercent: 67 });
    render(<RangeDisplay rangeMinMi={29} rangeMaxMi={40} />);

    // 29*33%=9.6→10, 40*33%=13.2→13 for current; 29*67%=19.4→19, 40*67%=26.8→27 for target
    expect(screen.getByText("10-13 mi")).toBeInTheDocument();
    expect(screen.getByText("19-27 mi")).toBeInTheDocument();
  });

  it("includes aria-labels for current and target ranges", () => {
    useGaugeStore.setState({ currentPercent: 50, targetPercent: 80 });
    render(<RangeDisplay rangeMinMi={29} rangeMaxMi={40} />);

    const currentEl = screen.getByText("15-20 mi");
    const targetEl = screen.getByText("23-32 mi");

    expect(currentEl).toHaveAttribute("aria-label", "Current range: 15-20 mi");
    expect(targetEl).toHaveAttribute("aria-label", "Target range: 23-32 mi");
  });
});
