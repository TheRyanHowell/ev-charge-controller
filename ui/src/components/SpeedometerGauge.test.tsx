import { useGaugeStore } from "@/stores/gaugeStore";
import {
  render,
  screen,
  fireEvent,
  cleanup,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import SpeedometerGauge from "./SpeedometerGauge";

// Mock requestAnimationFrame to execute synchronously in tests
const rafIds = new Map<number, (time: number) => void>();
let nextRafId = 1;
beforeEach(() => {
  globalThis.requestAnimationFrame = (cb) => {
    const id = nextRafId++;
    rafIds.set(id, cb);
    return id;
  };
  globalThis.cancelAnimationFrame = (id) => {
    rafIds.delete(id);
  };
  // Mock ResizeObserver to prevent async setCenterScale updates in tests
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
});
afterEach(() => {
  rafIds.clear();
});

function flushRaf() {
  const cbs = Array.from(rafIds.values());
  rafIds.clear();
  for (const cb of cbs) {
    cb(0);
  }
}

function flushRafLoop() {
  let iterations = 0;
  const maxIterations = 10;
  while (rafIds.size > 0 && iterations < maxIterations) {
    flushRaf();
    iterations++;
  }
}

describe("SpeedometerGauge", () => {
  const mockOnStartStop = vi.fn();

  const defaultProps = {
    startPercent: 25,
    currentPercent: 25,
    targetPercent: 80,
    status: "idle" as const,
    onStartStop: mockOnStartStop,
  };

  beforeEach(() => {
    mockOnStartStop.mockClear();
    useGaugeStore.setState({ currentPercent: 25, targetPercent: 80 });
  });

  // SVG viewBox is 300x300, arc radius is 107.1 (300 * 0.42 * 0.85)
  const VIEW_BOX = 300;
  const ARC_R = VIEW_BOX * 0.42 * 0.85;
  const CX = VIEW_BOX / 2;
  const CY = VIEW_BOX / 2;
  const START_ANGLE_DEG = 135;

  function svgPoint(pct: number) {
    const angleRad = ((START_ANGLE_DEG + (pct / 100) * 270) * Math.PI) / 180;
    return {
      x: CX + Math.cos(angleRad) * ARC_R,
      y: CY + Math.sin(angleRad) * ARC_R,
    };
  }

  function fireDragToPoint(element: Element, fromPct: number, toPct: number) {
    const from = svgPoint(fromPct);
    const to = svgPoint(toPct);
    fireEvent.pointerDown(element, { clientX: from.x, clientY: from.y });
    fireEvent.pointerMove(element, { clientX: to.x, clientY: to.y });
    act(() => {
      flushRafLoop();
    });
    fireEvent.pointerUp(element);
  }

  function getSvg(parent: ParentNode): SVGSVGElement {
    const svg = parent.querySelector("svg");
    expect(svg).toBeInTheDocument();
    return svg as SVGSVGElement;
  }

  it("renders correctly with default props", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    expect(screen.getByTestId("speedometer-gauge")).toBeInTheDocument();
    expect(screen.getByText("25%")).toBeInTheDocument();
    expect(screen.getByText(/READY/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /START/i })).toBeInTheDocument();
  });

  it("displays correct percentage", () => {
    render(<SpeedometerGauge {...defaultProps} currentPercent={50} />);
    expect(screen.getByText("50%")).toBeInTheDocument();
  });

  it("uses Math.floor for aria-valuetext percentage", () => {
    render(
      <SpeedometerGauge
        {...defaultProps}
        currentPercent={45.7}
        targetPercent={80.3}
      />,
    );
    const svg = screen.getByTestId("speedometer-gauge").querySelector("svg");
    expect(svg).toBeInTheDocument();
    const ariaValue = svg?.getAttribute("aria-valuetext");
    expect(ariaValue).toBe("Current 45%, target 80%");
  });

  it("shows chrome border ring", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    expect(gauge).toBeInTheDocument();
  });

  it("handles start/stop button click", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    const button = screen.getByRole("button", { name: /START/i });
    fireEvent.click(button);
    expect(mockOnStartStop).toHaveBeenCalledTimes(1);
  });

  it("handles gauge change for target", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);
    fireDragToPoint(svg, 80, 90);
    const state = useGaugeStore.getState();
    expect(state.currentPercent).toBe(25);
    expect(state.targetPercent).toBe(90);
  });

  it("handles gauge change for current", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);
    fireDragToPoint(svg, 25, 35);
    const state = useGaugeStore.getState();
    expect(state.currentPercent).toBeCloseTo(35, 2);
    expect(state.targetPercent).toBe(80);
  });

  it("displays charging status", () => {
    render(<SpeedometerGauge {...defaultProps} status="charging" />);
    expect(screen.getByText(/CHARGING/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /STOP/i })).toBeInTheDocument();
  });

  it("displays idle status", () => {
    render(<SpeedometerGauge {...defaultProps} status="idle" />);
    expect(screen.getByText(/READY/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /START/i })).toBeInTheDocument();
  });

  it("updates button text based on status", () => {
    const { rerender } = render(
      <SpeedometerGauge {...defaultProps} status="charging" />,
    );
    expect(screen.getByRole("button", { name: /STOP/i })).toBeInTheDocument();

    rerender(<SpeedometerGauge {...defaultProps} status="idle" />);
    expect(screen.getByRole("button", { name: /START/i })).toBeInTheDocument();
  });

  it("smoothly drags current marker from 25% to 50% position", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);
    fireDragToPoint(svg, 25, 50);
    const state = useGaugeStore.getState();
    expect(state.currentPercent).toBeCloseTo(50, 2);
    expect(state.targetPercent).toBe(80);
  });

  it("smoothly drags target marker from 80% to 60% position", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);
    fireDragToPoint(svg, 80, 60);
    const state = useGaugeStore.getState();
    expect(state.currentPercent).toBe(25);
    expect(state.targetPercent).toBe(60);
  });

  it("bumps target when dragging current past it", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);

    fireDragToPoint(svg, 25, 76);
    let state = useGaugeStore.getState();
    expect(state.currentPercent).toBeCloseTo(76, 2);
    expect(state.targetPercent).toBe(80);

    // Re-render with post-bump values so hit detection matches rendered positions
    cleanup();
    useGaugeStore.setState({ currentPercent: 76, targetPercent: 80 });
    render(<SpeedometerGauge {...defaultProps} startPercent={76} />);
    const gauge2 = screen.getByTestId("speedometer-gauge");
    const svg2 = getSvg(gauge2);

    // Drag past target - current bumps, target jumps to next 5% increment
    fireDragToPoint(svg2, 76, 85);
    state = useGaugeStore.getState();
    expect(state.currentPercent).toBe(85);
    expect(state.targetPercent).toBe(90);
  });

  it("bumps current when dragging target past it", () => {
    // Stage 1: drag target below current - both bump together
    useGaugeStore.setState({ currentPercent: 25, targetPercent: 30 });
    render(<SpeedometerGauge {...defaultProps} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);

    fireDragToPoint(svg, 30, 20);
    let state = useGaugeStore.getState();
    expect(state.currentPercent).toBe(20);
    expect(state.targetPercent).toBe(20);

    // Stage 2: render with post-bump values and drag target back above current
    // current should stay at the bumped value (20%), only target moves
    cleanup();
    useGaugeStore.setState({ currentPercent: 20, targetPercent: 25 });
    render(<SpeedometerGauge {...defaultProps} startPercent={20} />);
    const gauge2 = screen.getByTestId("speedometer-gauge");
    const svg2 = getSvg(gauge2);

    fireDragToPoint(svg2, 25, 50);
    state = useGaugeStore.getState();
    expect(state.currentPercent).toBe(20);
    expect(state.targetPercent).toBe(50);
  });

  it("bumps both markers when dragging current past target while they are equal", () => {
    // Regression: when current === target, dragging current past target should bump both
    // not cap current at target (the < vs <= edge case in bump condition)
    // Markers set at 79/80 (slightly apart) so current is grabbed, not target.
    useGaugeStore.setState({ currentPercent: 79, targetPercent: 80 });
    render(<SpeedometerGauge {...defaultProps} startPercent={79} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);

    // Drag current from 79% to 81% - current bumps, target jumps to next 5% (85%)
    fireDragToPoint(svg, 79, 81);
    const state = useGaugeStore.getState();
    expect(state.currentPercent).toBe(81);
    expect(state.targetPercent).toBe(85);
    expect(state.currentPercent).toBeGreaterThan(80);
  });

  it("never allows current to exceed target after drag", () => {
    // Regression: current must never exceed target regardless of drag direction
    useGaugeStore.setState({ currentPercent: 80, targetPercent: 80 });
    render(<SpeedometerGauge {...defaultProps} startPercent={80} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);

    // Try to drag current past target - bump fires, target jumps to next 5%
    fireDragToPoint(svg, 80, 81);

    const state = useGaugeStore.getState();
    expect(state.currentPercent).toBeLessThanOrEqual(state.targetPercent);
  });

  it("updates target marker when dragging", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);

    fireDragToPoint(svg, 80, 60);
    const state = useGaugeStore.getState();
    expect(state.currentPercent).toBe(25);
    expect(state.targetPercent).toBeCloseTo(60, 2);
  });

  it("maintains current marker drag when moving slightly off marker on speedometer gauge", () => {
    render(<SpeedometerGauge {...defaultProps} />);
    const gauge = screen.getByTestId("speedometer-gauge");
    const svg = getSvg(gauge);

    // Click on current marker at 25%
    const marker = svgPoint(25);
    fireEvent.pointerDown(svg, { clientX: marker.x, clientY: marker.y });

    // Move slightly off the marker vertically - should still drag current marker
    fireEvent.pointerMove(svg, { clientX: marker.x, clientY: marker.y - 8 });
    act(() => {
      flushRafLoop();
    });

    const state = useGaugeStore.getState();
    expect(state.currentPercent).toBeGreaterThan(20);
    expect(state.currentPercent).toBeLessThan(30);
  });

  describe("Charging state", () => {
    beforeEach(() => {
      useGaugeStore.setState({ currentPercent: 40, targetPercent: 80 });
    });

    const chargingProps = {
      ...defaultProps,
      status: "charging" as const,
    };

    it("displays start marker when charging", () => {
      render(<SpeedometerGauge {...chargingProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);
      const circles = svg.querySelectorAll("circle");
      expect(circles.length).toBeGreaterThan(0);
    });

    it("does not allow dragging current marker when charging", () => {
      useGaugeStore.setState({ currentPercent: 40, targetPercent: 80 });
      render(<SpeedometerGauge {...chargingProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireDragToPoint(svg, 30, 40);

      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(40);
      expect(state.targetPercent).toBe(80);
    });

    it("allows dragging target marker when charging, but clamps to current minimum", () => {
      render(<SpeedometerGauge {...chargingProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireDragToPoint(svg, 80, 90);

      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(40);
      expect(state.targetPercent).toBe(90);
    });

    it("clamps target to latest current value when props change mid-drag", async () => {
      // Regression: when dragging target during charging and props change between
      // mousedown and mouseup, the clamping must use the latest current value (from store),
      // not the stale closure value. Previously this caused target to go below current.
      useGaugeStore.setState({ currentPercent: 71, targetPercent: 90 });
      const { rerender } = render(
        <SpeedometerGauge
          {...defaultProps}
          status="charging"
          startPercent={45}
        />,
      );
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      await act(async () => {
        // Start drag on target at 90%
        const from = svgPoint(90);
        fireEvent.pointerDown(svg, { clientX: from.x, clientY: from.y });

        // Simulate parent updating currentPercent mid-drag (e.g., from polling)
        useGaugeStore.setState({ currentPercent: 72 });
        rerender(
          <SpeedometerGauge
            {...defaultProps}
            status="charging"
            startPercent={45}
          />,
        );
        flushRafLoop();

        // Complete drag to 50% - target should be clamped to 72% (latest value)
        const to = svgPoint(50);
        fireEvent.pointerMove(svg, { clientX: to.x, clientY: to.y });
        flushRafLoop();
        fireEvent.pointerUp(svg);
      });

      // Target must NOT go below current (72%)
      const state = useGaugeStore.getState();
      expect(state.targetPercent).toBeGreaterThanOrEqual(72);
    });
  });

  describe("Bump edge cases", () => {
    it("does not bump when dragging current exactly to target", () => {
      useGaugeStore.setState({ currentPercent: 25, targetPercent: 50 });
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireDragToPoint(svg, 25, 50);
      // Should cap at target, not bump
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(50);
      expect(state.targetPercent).toBe(50);
    });

    it("bumps target to next 5% when current is just below a 5% increment", () => {
      useGaugeStore.setState({ currentPercent: 25, targetPercent: 50 });
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // Drag current to 54% - just below 55%, should bump target to 55%
      fireDragToPoint(svg, 25, 54);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(54);
      expect(state.targetPercent).toBe(55);
    });

    it("caps current when dragging backward past target", () => {
      useGaugeStore.setState({ currentPercent: 80, targetPercent: 60 });
      render(<SpeedometerGauge {...defaultProps} startPercent={80} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireDragToPoint(svg, 80, 50);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(50);
      expect(state.targetPercent).toBe(60);
    });

    it("does not bump when target is already at 100%", () => {
      useGaugeStore.setState({ currentPercent: 50, targetPercent: 100 });
      render(<SpeedometerGauge {...defaultProps} startPercent={50} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireDragToPoint(svg, 50, 95);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(95);
      expect(state.targetPercent).toBe(100);
    });

    it("handles sequential drags: bump then drag target independently", () => {
      // Stage 1: drag current past target - bump fires
      useGaugeStore.setState({ currentPercent: 49, targetPercent: 50 });
      render(<SpeedometerGauge {...defaultProps} startPercent={49} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireDragToPoint(svg, 49, 55);
      let state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(55);
      expect(state.targetPercent).toBe(60);

      // Stage 2: render with post-bump values and drag target independently
      cleanup();
      useGaugeStore.setState({ currentPercent: 55, targetPercent: 60 });
      render(<SpeedometerGauge {...defaultProps} startPercent={55} />);
      const gauge2 = screen.getByTestId("speedometer-gauge");
      const svg2 = getSvg(gauge2);

      fireDragToPoint(svg2, 60, 80);
      state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(55);
      expect(state.targetPercent).toBe(80);
    });

    it("bumps when dragging current from below target to above", () => {
      useGaugeStore.setState({ currentPercent: 48, targetPercent: 50 });
      render(<SpeedometerGauge {...defaultProps} startPercent={48} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireDragToPoint(svg, 48, 52);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(52);
      expect(state.targetPercent).toBe(55);
    });

    it("does not fire bump when dragging current from above target downward", () => {
      useGaugeStore.setState({ currentPercent: 60, targetPercent: 50 });
      render(<SpeedometerGauge {...defaultProps} startPercent={60} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireDragToPoint(svg, 60, 55);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(50);
      expect(state.targetPercent).toBe(50);
    });

    it("bumps target to 100% when dragging current near 100", () => {
      useGaugeStore.setState({ currentPercent: 89, targetPercent: 90 });
      render(<SpeedometerGauge {...defaultProps} startPercent={89} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireDragToPoint(svg, 89, 97);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(97);
      expect(state.targetPercent).toBe(100);
    });
  });

  describe("Error status", () => {
    it("renders ERROR text when status is error", () => {
      render(<SpeedometerGauge {...defaultProps} status="error" />);
      expect(screen.getByText(/ERROR/i)).toBeInTheDocument();
    });

    it("renders START button when status is error", () => {
      render(<SpeedometerGauge {...defaultProps} status="error" />);
      expect(
        screen.getByRole("button", { name: /START/i }),
      ).toBeInTheDocument();
    });
  });

  describe("Button states", () => {
    it("renders CHARGED button when current >= target and not charging", () => {
      render(
        <SpeedometerGauge
          {...defaultProps}
          currentPercent={80}
          targetPercent={80}
          status="idle"
        />,
      );
      expect(
        screen.getByRole("button", { name: /CHARGED/i }),
      ).toBeInTheDocument();
    });

    it("disables button when current >= target and not charging", () => {
      render(
        <SpeedometerGauge
          {...defaultProps}
          currentPercent={80}
          targetPercent={80}
          status="idle"
        />,
      );
      const button = screen.getByRole("button", { name: /CHARGED/i });
      expect(button).toBeDisabled();
    });

    it("renders STOP button when charging (even if current === target)", () => {
      render(
        <SpeedometerGauge
          {...defaultProps}
          currentPercent={80}
          targetPercent={80}
          status="charging"
        />,
      );
      expect(screen.getByRole("button", { name: /STOP/i })).toBeInTheDocument();
    });

    it("does not disable button when charging", () => {
      render(
        <SpeedometerGauge
          {...defaultProps}
          currentPercent={80}
          targetPercent={80}
          status="charging"
        />,
      );
      const button = screen.getByRole("button", { name: /STOP/i });
      expect(button).not.toBeDisabled();
    });
  });

  describe("SVG structure", () => {
    it("renders 11 tick label <text> elements (0-100)", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);
      const textElements = svg.querySelectorAll("text");
      expect(textElements.length).toBe(11);
    });

    it("renders green progress arc only when charging", () => {
      const { container: charging } = render(
        <SpeedometerGauge {...defaultProps} status="charging" />,
      );
      const svgCharging = getSvg(charging);
      const greenArcsCharging = svgCharging.querySelectorAll("path[d]");
      // background arc + progress arc = at least 2 paths
      expect(greenArcsCharging.length).toBeGreaterThanOrEqual(2);

      const { container: idle } = render(
        <SpeedometerGauge {...defaultProps} status="idle" />,
      );
      const svgIdle = getSvg(idle);
      // background arc only = 1 path (no green progress arc)
      const greenArcsIdle = svgIdle.querySelectorAll("path[d]");
      expect(greenArcsIdle.length).toBeGreaterThanOrEqual(1);
    });

    it("renders green start marker only when charging", () => {
      const { getByTestId } = render(
        <SpeedometerGauge {...defaultProps} status="charging" />,
      );
      const gauge = getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);
      // Start marker is a green circle (fill="#22c55e")
      const greenCircles = svg.querySelectorAll('circle[fill="#22c55e"]');
      expect(greenCircles.length).toBeGreaterThan(0);
    });
  });

  describe("mouseLeave handling", () => {
    it("clears draggingGauge on mouse leave", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // Start dragging at 25%
      fireDragToPoint(svg, 25, 30);
      let state = useGaugeStore.getState();
      expect(state.currentPercent).toBeGreaterThan(25);

      const prevState = useGaugeStore.getState();
      // Mouse leave should clear dragging state
      fireEvent.pointerLeave(svg);

      // Subsequent mouseMove should not change store
      fireEvent.pointerMove(svg, { clientX: 150, clientY: 150 });
      state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(prevState.currentPercent);
      expect(state.targetPercent).toBe(prevState.targetPercent);
    });
  });

  describe("global drag ending in gap", () => {
    it("clears draggingGauge when mouseup occurs in the gap", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // The gap is at the top-right (above the 270-degree arc end at 405 degrees)
      // At 0% the angle is 135 degrees (bottom-left). At 100% it's 405 degrees (bottom-right).
      // The gap spans from 405 to 360+135=495 degrees (wrapping around the top)
      // A point just past the arc end (e.g., 101%) lands in the gap
      const gapPt = svgPoint(101);
      fireEvent.pointerDown(svg, { clientX: gapPt.x, clientY: gapPt.y });
      fireEvent.pointerMove(svg, { clientX: gapPt.x, clientY: gapPt.y });
      act(() => {
        flushRafLoop();
      });
      fireEvent.pointerUp(svg);

      // Should not change store when ending in the gap
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(25);
      expect(state.targetPercent).toBe(80);
    });

    it("applies drag value via global window mouseup when dragging and releasing outside SVG", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // Start drag on target at 80%
      const from = svgPoint(80);
      fireEvent.pointerDown(svg, { clientX: from.x, clientY: from.y });

      // Move to 90% - this triggers raf-based applyDragFromAngle
      const to = svgPoint(90);
      fireEvent.pointerMove(svg, { clientX: to.x, clientY: to.y });
      act(() => {
        flushRafLoop();
      });

      // Release on window (not SVG) - triggers global onUp handler
      act(() => {
        fireEvent.pointerUp(document.documentElement);
      });

      const state = useGaugeStore.getState();
      expect(state.targetPercent).toBe(90);
    });

    it("clears dragging via global window mouseup when releasing in gap", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // Start drag on target at 80%
      const from = svgPoint(80);
      fireEvent.pointerDown(svg, { clientX: from.x, clientY: from.y });

      // Move into the gap (101% is past the arc end)
      const gapPt = svgPoint(101);
      fireEvent.pointerMove(svg, { clientX: gapPt.x, clientY: gapPt.y });
      act(() => {
        flushRafLoop();
      });

      // Release on window - triggers global onUp, should clear without applying
      act(() => {
        fireEvent.pointerUp(document.documentElement);
      });

      const state = useGaugeStore.getState();
      // Target should remain unchanged since release was in the gap
      expect(state.targetPercent).toBe(80);
    });

    it("clears dragging via SVG-level mouseup when releasing in gap", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // Start drag on target at 80%
      const from = svgPoint(80);
      fireEvent.pointerDown(svg, { clientX: from.x, clientY: from.y });

      // Move into the gap (101% is past the arc end)
      const gapPt = svgPoint(101);
      fireEvent.pointerMove(svg, { clientX: gapPt.x, clientY: gapPt.y });
      act(() => {
        flushRafLoop();
      });

      // Release on SVG - triggers SVG-level handleSvgMouseUp
      fireEvent.pointerUp(svg);

      const state = useGaugeStore.getState();
      // Target should remain unchanged since release was in the gap
      expect(state.targetPercent).toBe(80);
    });

    it("fires onDragEnd via mouseLeave when dragging and moved", () => {
      const onDragEnd = vi.fn();
      render(<SpeedometerGauge {...defaultProps} onDragEnd={onDragEnd} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // Start drag on target at 80%
      const from = svgPoint(80);
      fireEvent.pointerDown(svg, { clientX: from.x, clientY: from.y });

      // Move to 90% so dragMovedRef is set
      const to = svgPoint(90);
      fireEvent.pointerMove(svg, { clientX: to.x, clientY: to.y });
      act(() => {
        flushRafLoop();
      });

      // Leave the SVG - should fire onDragEnd
      fireEvent.pointerLeave(svg);

      expect(onDragEnd).toHaveBeenCalled();
    });

    it("fires onDragEnd via SVG-level mouseup when dragging, moved, and released in gap", () => {
      const onDragEnd = vi.fn();
      render(<SpeedometerGauge {...defaultProps} onDragEnd={onDragEnd} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // Start drag on target at 80%
      const from = svgPoint(80);
      fireEvent.pointerDown(svg, { clientX: from.x, clientY: from.y });

      // Move to 85% first (valid position, sets dragMovedRef = true)
      const midPt = svgPoint(85);
      fireEvent.pointerMove(svg, { clientX: midPt.x, clientY: midPt.y });
      act(() => {
        flushRafLoop();
      });

      // Then move into the gap (101% is past the arc end)
      const gapPt = svgPoint(101);
      fireEvent.pointerMove(svg, { clientX: gapPt.x, clientY: gapPt.y });
      act(() => {
        flushRafLoop();
      });

      // Release on SVG - should fire onDragEnd since moved=true
      fireEvent.pointerUp(svg);

      expect(onDragEnd).toHaveBeenCalled();
    });
  });

  describe("touch events", () => {
    function fireTouchDragToPoint(
      element: Element,
      fromPct: number,
      toPct: number,
    ) {
      const from = svgPoint(fromPct);
      const to = svgPoint(toPct);
      fireEvent.pointerDown(element, { clientX: from.x, clientY: from.y });
      fireEvent.pointerMove(element, { clientX: to.x, clientY: to.y });
      act(() => {
        flushRafLoop();
      });
      fireEvent.pointerUp(element);
    }

    it("drags target marker with touch events", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);
      fireTouchDragToPoint(svg, 80, 90);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(25);
      expect(state.targetPercent).toBe(90);
    });

    it("drags current marker with touch events", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);
      fireTouchDragToPoint(svg, 25, 35);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBeCloseTo(35, 2);
      expect(state.targetPercent).toBe(80);
    });

    it("releases marker on touchend", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      const from = svgPoint(80);
      fireEvent.pointerDown(svg, { clientX: from.x, clientY: from.y });

      const to = svgPoint(60);
      fireEvent.pointerMove(svg, { clientX: to.x, clientY: to.y });
      act(() => {
        flushRafLoop();
      });
      fireEvent.pointerUp(svg);

      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(25);
      expect(state.targetPercent).toBe(60);
    });

    it("does not allow dragging current marker when charging via touch", () => {
      useGaugeStore.setState({ currentPercent: 40, targetPercent: 80 });
      render(<SpeedometerGauge {...defaultProps} status="charging" />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireTouchDragToPoint(svg, 30, 40);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(40);
      expect(state.targetPercent).toBe(80);
    });

    it("allows dragging target marker when charging via touch", () => {
      useGaugeStore.setState({ currentPercent: 40, targetPercent: 80 });
      render(<SpeedometerGauge {...defaultProps} status="charging" />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireTouchDragToPoint(svg, 80, 90);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(40);
      expect(state.targetPercent).toBe(90);
    });

    it("bumps target when dragging current past it via touch", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireTouchDragToPoint(svg, 25, 76);
      let state = useGaugeStore.getState();
      expect(state.currentPercent).toBeCloseTo(76, 2);
      expect(state.targetPercent).toBe(80);

      // Re-render with post-bump values so hit detection matches rendered positions
      cleanup();
      useGaugeStore.setState({ currentPercent: 76, targetPercent: 80 });
      render(<SpeedometerGauge {...defaultProps} startPercent={76} />);
      const gauge2 = screen.getByTestId("speedometer-gauge");
      const svg2 = getSvg(gauge2);

      fireTouchDragToPoint(svg2, 76, 85);
      state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(85);
      expect(state.targetPercent).toBe(90);
    });
  });

  describe("hit detection regression", () => {
    // Regression: angularDistance() did not normalize angles to [0, 2π) before
    // comparing. percentageToAngle returns [START_ANGLE, START_ANGLE + TOTAL_ARC]
    // (up to 405°) while getAngleFromOffset returns [0, 2π). Angles above 360°
    // (96-100%) had wraparound errors - e.g. 405° vs 45° gave d=360° → 0,
    // making 84% appear to hit 100%.
    it("does not grab target at 100% when clicking at 84%", () => {
      useGaugeStore.setState({ currentPercent: 20, targetPercent: 100 });
      render(<SpeedometerGauge {...defaultProps} startPercent={20} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // 84% is 43.2° away from 100% - well outside 6° hit radius
      fireDragToPoint(svg, 84, 88);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(20);
      expect(state.targetPercent).toBe(100);
    });

    it("grabs target at 100% when clicking at 98%", () => {
      useGaugeStore.setState({ currentPercent: 20, targetPercent: 100 });
      render(<SpeedometerGauge {...defaultProps} startPercent={20} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // 98% is 5.4° away from 100% - within 6° hit radius
      fireDragToPoint(svg, 98, 96);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(20);
      expect(state.targetPercent).toBe(96);
    });

    it("does not grab target at 96% when clicking at 84%", () => {
      useGaugeStore.setState({ currentPercent: 20, targetPercent: 96 });
      render(<SpeedometerGauge {...defaultProps} startPercent={20} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // 84% is 32.4° away from 96% - well outside 6° hit radius
      fireDragToPoint(svg, 84, 88);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(20);
      expect(state.targetPercent).toBe(96);
    });

    it("correctly computes distance for markers near 0%/100% boundary", () => {
      // 0% and 100% are 135° apart in the gap (not adjacent on the arc)
      useGaugeStore.setState({ currentPercent: 0, targetPercent: 100 });
      render(<SpeedometerGauge {...defaultProps} startPercent={0} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      // Clicking at 2% should grab start (0%), not target (100%)
      fireDragToPoint(svg, 2, 5);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(5);
      expect(state.targetPercent).toBe(100);
    });
  });

  describe("regression: button click with pointer-events overlay", () => {
    it("start/stop button has pointer-events auto to override GaugeOverlay none", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const button = screen.getByRole("button", { name: /START/i });
      expect(button.style.pointerEvents).toBe("auto");
    });

    it("start/stop button click fires onStartStop", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const button = screen.getByRole("button", { name: /START/i });
      fireEvent.click(button);
      expect(mockOnStartStop).toHaveBeenCalledTimes(1);
    });
  });

  describe("regression: click without drag does not fire onDragEnd", () => {
    it("mousedown + mouseup without mousemove does not change store", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);
      const pt = svgPoint(50);
      fireEvent.pointerDown(svg, { clientX: pt.x, clientY: pt.y });
      fireEvent.pointerUp(svg);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(25);
      expect(state.targetPercent).toBe(80);
    });
  });

  describe("keyboard navigation", () => {
    it("increases target on ArrowUp", () => {
      useGaugeStore.setState({ currentPercent: 25, targetPercent: 78 });
      const { container } = render(<SpeedometerGauge {...defaultProps} />);
      const svg = getSvg(container);
      fireEvent.keyDown(svg, { key: "ArrowUp", preventDefault: () => {} });
      expect(useGaugeStore.getState().targetPercent).toBe(80);
    });

    it("decreases target on ArrowDown", () => {
      useGaugeStore.setState({ currentPercent: 25, targetPercent: 82 });
      const { container } = render(<SpeedometerGauge {...defaultProps} />);
      const svg = getSvg(container);
      fireEvent.keyDown(svg, { key: "ArrowDown", preventDefault: () => {} });
      expect(useGaugeStore.getState().targetPercent).toBe(80);
    });

    it("increases current on Shift+ArrowUp when idle", () => {
      useGaugeStore.setState({ currentPercent: 23, targetPercent: 80 });
      const { container } = render(<SpeedometerGauge {...defaultProps} />);
      const svg = getSvg(container);
      fireEvent.keyDown(svg, {
        key: "ArrowUp",
        shiftKey: true,
        preventDefault: () => {},
      });
      expect(useGaugeStore.getState().currentPercent).toBe(25);
    });

    it("decreases current on Shift+ArrowDown when idle", () => {
      useGaugeStore.setState({ currentPercent: 27, targetPercent: 80 });
      const { container } = render(<SpeedometerGauge {...defaultProps} />);
      const svg = getSvg(container);
      fireEvent.keyDown(svg, {
        key: "ArrowDown",
        shiftKey: true,
        preventDefault: () => {},
      });
      expect(useGaugeStore.getState().currentPercent).toBe(25);
    });

    it("does not change current on Shift+ArrowUp when charging", () => {
      useGaugeStore.setState({ currentPercent: 25, targetPercent: 80 });
      const { container } = render(
        <SpeedometerGauge {...defaultProps} status="charging" />,
      );
      const svg = getSvg(container);
      fireEvent.keyDown(svg, {
        key: "ArrowUp",
        shiftKey: true,
        preventDefault: () => {},
      });
      expect(useGaugeStore.getState().currentPercent).toBe(25);
    });

    it("clamps target to 100% on ArrowUp", () => {
      const { container } = render(
        <SpeedometerGauge
          {...defaultProps}
          currentPercent={25}
          targetPercent={100}
        />,
      );
      const svg = getSvg(container);
      fireEvent.keyDown(svg, { key: "ArrowUp", preventDefault: () => {} });
      expect(useGaugeStore.getState().targetPercent).toBe(100);
    });

    it("clamps target to 0% on ArrowDown", () => {
      const { container } = render(
        <SpeedometerGauge
          {...defaultProps}
          currentPercent={25}
          targetPercent={0}
        />,
      );
      const svg = getSvg(container);
      fireEvent.keyDown(svg, { key: "ArrowDown", preventDefault: () => {} });
      expect(useGaugeStore.getState().targetPercent).toBe(0);
    });

    it("clamps target to currentPercent when charging on ArrowDown", () => {
      const { container } = render(
        <SpeedometerGauge
          {...defaultProps}
          currentPercent={50}
          targetPercent={53}
          status="charging"
        />,
      );
      const svg = getSvg(container);
      fireEvent.keyDown(svg, { key: "ArrowDown", preventDefault: () => {} });
      expect(useGaugeStore.getState().targetPercent).toBe(50);
    });

    it("increases target on ArrowRight", () => {
      useGaugeStore.setState({ currentPercent: 25, targetPercent: 78 });
      const { container } = render(<SpeedometerGauge {...defaultProps} />);
      const svg = getSvg(container);
      fireEvent.keyDown(svg, { key: "ArrowRight", preventDefault: () => {} });
      expect(useGaugeStore.getState().targetPercent).toBe(80);
    });

    it("decreases target on ArrowLeft", () => {
      useGaugeStore.setState({ currentPercent: 25, targetPercent: 82 });
      const { container } = render(<SpeedometerGauge {...defaultProps} />);
      const svg = getSvg(container);
      fireEvent.keyDown(svg, { key: "ArrowLeft", preventDefault: () => {} });
      expect(useGaugeStore.getState().targetPercent).toBe(80);
    });
  });

  describe("keyboard commit (persistence)", () => {
    it("commits the settled values after a debounce", () => {
      vi.useFakeTimers();
      try {
        useGaugeStore.setState({ currentPercent: 25, targetPercent: 78 });
        const onDragEnd = vi.fn();
        const { container } = render(
          <SpeedometerGauge {...defaultProps} onDragEnd={onDragEnd} />,
        );
        const svg = getSvg(container);
        fireEvent.keyDown(svg, { key: "ArrowUp", preventDefault: () => {} });
        // Debounced - not committed immediately.
        expect(onDragEnd).not.toHaveBeenCalled();
        act(() => {
          vi.advanceTimersByTime(300);
        });
        expect(onDragEnd).toHaveBeenCalledWith(25, 80);
      } finally {
        vi.useRealTimers();
      }
    });

    it("coalesces rapid arrow keys into a single commit", () => {
      vi.useFakeTimers();
      try {
        useGaugeStore.setState({ currentPercent: 25, targetPercent: 70 });
        const onDragEnd = vi.fn();
        const { container } = render(
          <SpeedometerGauge {...defaultProps} onDragEnd={onDragEnd} />,
        );
        const svg = getSvg(container);
        for (let i = 0; i < 5; i++) {
          fireEvent.keyDown(svg, { key: "ArrowUp", preventDefault: () => {} });
        }
        act(() => {
          vi.advanceTimersByTime(300);
        });
        expect(onDragEnd).toHaveBeenCalledTimes(1);
      } finally {
        vi.useRealTimers();
      }
    });
  });

  describe("tasmota button disable", () => {
    it("disables START button when tasmotaConnected is false", () => {
      render(<SpeedometerGauge {...defaultProps} tasmotaConnected={false} />);
      const button = screen.getByRole("button", { name: /START/i });
      expect(button).toBeDisabled();
    });

    it("does not disable START button when tasmotaConnected is true", () => {
      render(<SpeedometerGauge {...defaultProps} tasmotaConnected={true} />);
      const button = screen.getByRole("button", { name: /START/i });
      expect(button).not.toBeDisabled();
    });

    it("does not disable START button when tasmotaConnected is null (unknown)", () => {
      render(<SpeedometerGauge {...defaultProps} tasmotaConnected={null} />);
      const button = screen.getByRole("button", { name: /START/i });
      expect(button).not.toBeDisabled();
    });

    it("does not disable STOP button when tasmotaConnected is false while charging", () => {
      render(
        <SpeedometerGauge
          {...defaultProps}
          status="charging"
          tasmotaConnected={false}
        />,
      );
      const button = screen.getByRole("button", { name: /STOP/i });
      expect(button).not.toBeDisabled();
    });
  });

  describe("scroll lock during drag", () => {
    beforeEach(() => {
      document.documentElement.style.overflow = "";
    });

    afterEach(() => {
      document.documentElement.style.overflow = "";
    });

    it("locks body scroll only for touch pointerType", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      const from = svgPoint(80);
      fireEvent.pointerDown(svg, {
        clientX: from.x,
        clientY: from.y,
        pointerType: "touch",
      });

      expect(document.documentElement.style.overflow).toBe("hidden");
    });

    it("does not lock scroll for mouse pointerType", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      const from = svgPoint(80);
      fireEvent.pointerDown(svg, {
        clientX: from.x,
        clientY: from.y,
        pointerType: "mouse",
      });

      expect(document.documentElement.style.overflow).toBe("");
    });

    it("restores body scroll when touch drag ends via pointerUp", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      const from = svgPoint(80);
      fireEvent.pointerDown(svg, {
        clientX: from.x,
        clientY: from.y,
        pointerType: "touch",
      });
      expect(document.documentElement.style.overflow).toBe("hidden");

      const to = svgPoint(60);
      fireEvent.pointerMove(svg, { clientX: to.x, clientY: to.y });
      act(() => {
        flushRafLoop();
      });
      fireEvent.pointerUp(svg);

      expect(document.documentElement.style.overflow).toBe("");
    });

    it("restores body scroll when touch drag is cancelled via pointerLeave", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      const from = svgPoint(80);
      fireEvent.pointerDown(svg, {
        clientX: from.x,
        clientY: from.y,
        pointerType: "touch",
      });
      expect(document.documentElement.style.overflow).toBe("hidden");

      fireEvent.pointerLeave(svg);

      expect(document.documentElement.style.overflow).toBe("");
    });

    it("does not lock scroll when touching outside markers", () => {
      render(<SpeedometerGauge {...defaultProps} />);
      const gauge = screen.getByTestId("speedometer-gauge");
      const svg = getSvg(gauge);

      fireEvent.pointerDown(svg, {
        clientX: 0,
        clientY: 0,
        pointerType: "touch",
      });

      expect(document.documentElement.style.overflow).toBe("");
    });
  });
});
