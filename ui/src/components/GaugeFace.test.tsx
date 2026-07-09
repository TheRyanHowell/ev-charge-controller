import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { GaugeFace } from "./GaugeFace";

// Helper: extract SVG path elements from rendered output
function getPaths(container: HTMLElement) {
  return Array.from(container.querySelectorAll("path"));
}

// Helper: find the green charging arc path
function getChargingArc(container: HTMLElement) {
  const paths = getPaths(container);
  return paths.find((p) => p.getAttribute("stroke") === "#22c55e");
}

// Helper: parse SVG arc command from path d attribute
// Returns { fromX, fromY, toX, toY, radius, largeArc, sweep }
function parseArcCommand(d: string | null | undefined) {
  if (!d) return null;
  const match = d.match(
    /M\s+([\d.]+)\s+([\d.]+)\s+A\s+([\d.]+)\s+([\d.]+)\s+\d\s+(\d)\s+(\d)\s+([\d.]+)\s+([\d.]+)/,
  );
  if (!match) return null;
  const fromX = match[1] || "0";
  const fromY = match[2] || "0";
  const rx = match[3] || "0";
  const ry = match[4] || "0";
  const largeArc = match[5] || "0";
  const sweep = match[6] || "0";
  const toX = match[7] || "0";
  const toY = match[8] || "0";
  return {
    fromX: parseFloat(fromX),
    fromY: parseFloat(fromY),
    rx: parseFloat(rx),
    ry: parseFloat(ry),
    largeArc: parseInt(largeArc, 10),
    sweep: parseInt(sweep, 10),
    toX: parseFloat(toX),
    toY: parseFloat(toY),
  };
}

function getArcCommand(arc: SVGElement | null | undefined) {
  const d = arc?.getAttribute("d");
  const cmd = parseArcCommand(d);
  if (!cmd) throw new Error("Could not parse arc command");
  return cmd;
}

beforeEach(() => {
  // Reset gauge store
});

afterEach(() => {
  // Cleanup
});

describe("GaugeFace", () => {
  describe("charging arc rendering", () => {
    it("renders charging arc when status is charging", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      expect(arc).toBeTruthy();
    });

    it("does not render charging arc when idle", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="idle" />,
      );
      const arc = getChargingArc(container);
      expect(arc).toBeFalsy();
    });

    it("does not render charging arc when error", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="error" />,
      );
      const arc = getChargingArc(container);
      expect(arc).toBeFalsy();
    });

    it("does not render charging arc when pending", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="pending" />,
      );
      const arc = getChargingArc(container);
      expect(arc).toBeFalsy();
    });
  });

  describe("arc sweep direction", () => {
    it("sweeps clockwise (sweep=1) when currentPercent > startPercent", () => {
      const { container } = render(
        <GaugeFace currentPercent={80} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.sweep).toBe(1);
    });

    it("sweeps counter-clockwise (sweep=0) when currentPercent < startPercent", () => {
      const { container } = render(
        <GaugeFace currentPercent={20} startPercent={80} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.sweep).toBe(0);
    });

    it("sweeps clockwise (sweep=1) when currentPercent === startPercent", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={50} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.sweep).toBe(1);
    });

    it("sweeps clockwise for small progress (20 -> 25)", () => {
      const { container } = render(
        <GaugeFace currentPercent={25} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.sweep).toBe(1);
    });

    it("sweeps counter-clockwise for regression (80 -> 75)", () => {
      const { container } = render(
        <GaugeFace currentPercent={75} startPercent={80} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.sweep).toBe(0);
    });
  });

  describe("arc largeArc flag", () => {
    // The gauge spans 270°. largeArc=1 when arc > 180°, i.e. span > (180/270)*100 ≈ 66.67%.

    it("uses largeArc=0 for small span (20 -> 50, span=30)", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.largeArc).toBe(0); // 30/100*270=81° < 180°
    });

    it("uses largeArc=0 for span=50 (135°, below 180° threshold)", () => {
      const { container } = render(
        <GaugeFace currentPercent={70} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.largeArc).toBe(0); // 50/100*270=135° < 180°
    });

    it("uses largeArc=0 for span=60 (162°, still below 180° threshold)", () => {
      const { container } = render(
        <GaugeFace currentPercent={80} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.largeArc).toBe(0); // 60/100*270=162° < 180°
    });

    it("uses largeArc=0 for span=66 (178.2°, just below 180° threshold)", () => {
      const { container } = render(
        <GaugeFace currentPercent={86} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.largeArc).toBe(0); // 66/100*270=178.2° < 180°
    });

    it("uses largeArc=1 for span=67 (180.9°, just above 180° threshold)", () => {
      const { container } = render(
        <GaugeFace currentPercent={87} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.largeArc).toBe(1); // 67/100*270=180.9° > 180°
    });

    it("uses largeArc=1 for near-full arc (10 -> 95, span=85)", () => {
      const { container } = render(
        <GaugeFace currentPercent={95} startPercent={10} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.largeArc).toBe(1); // 85/100*270=229.5° > 180°
    });
  });

  describe("arc endpoint coordinates", () => {
    it("arc endpoints are within valid gauge bounds", () => {
      const { container } = render(
        <GaugeFace currentPercent={60} startPercent={30} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      // Gauge viewBox is 300x300, center is 150,150
      // All coordinates should be within 0-300
      expect(cmd.fromX).toBeGreaterThanOrEqual(0);
      expect(cmd.fromX).toBeLessThanOrEqual(300);
      expect(cmd.fromY).toBeGreaterThanOrEqual(0);
      expect(cmd.fromY).toBeLessThanOrEqual(300);
      expect(cmd.toX).toBeGreaterThanOrEqual(0);
      expect(cmd.toX).toBeLessThanOrEqual(300);
      expect(cmd.toY).toBeGreaterThanOrEqual(0);
      expect(cmd.toY).toBeLessThanOrEqual(300);
    });

    it("arc radius matches expected progress radius", () => {
      const { container } = render(
        <GaugeFace currentPercent={60} startPercent={30} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      // PROGRESS_R = 300 * 0.42 * 0.85 = 107.1
      const expectedRadius = 300 * 0.42 * 0.85;
      expect(cmd.rx).toBeCloseTo(expectedRadius, 1);
      expect(cmd.ry).toBeCloseTo(expectedRadius, 1);
    });

    it("arc start point matches startPercent position", () => {
      const { container } = render(
        <GaugeFace currentPercent={60} startPercent={0} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      // At 0%, the angle is 135 degrees
      const angle = (135 * Math.PI) / 180;
      const cx = 150;
      const cy = 150;
      const r = 300 * 0.42 * 0.85;
      const expectedX = cx + Math.cos(angle) * r;
      const expectedY = cy + Math.sin(angle) * r;
      expect(cmd.fromX).toBeCloseTo(expectedX, 1);
      expect(cmd.fromY).toBeCloseTo(expectedY, 1);
    });

    it("arc end point matches currentPercent position", () => {
      const { container } = render(
        <GaugeFace currentPercent={100} startPercent={30} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      // At 100%, the angle is 135 + 270 = 405 degrees
      const angle = (405 * Math.PI) / 180;
      const cx = 150;
      const cy = 150;
      const r = 300 * 0.42 * 0.85;
      const expectedX = cx + Math.cos(angle) * r;
      const expectedY = cy + Math.sin(angle) * r;
      expect(cmd.toX).toBeCloseTo(expectedX, 1);
      expect(cmd.toY).toBeCloseTo(expectedY, 1);
    });
  });

  describe("edge cases", () => {
    it("handles zero progress (start === current)", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={50} status="charging" />,
      );
      const arc = getChargingArc(container);
      expect(arc).toBeTruthy();
      const cmd = getArcCommand(arc);
      // Start and end should be the same point
      expect(cmd.fromX).toBeCloseTo(cmd.toX, 2);
      expect(cmd.fromY).toBeCloseTo(cmd.toY, 2);
    });

    it("handles full gauge arc (0 -> 100)", () => {
      const { container } = render(
        <GaugeFace currentPercent={100} startPercent={0} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.sweep).toBe(1);
      expect(cmd.largeArc).toBe(1); // span = 100 → 270° > 180°
    });

    it("handles reverse full arc (100 -> 0)", () => {
      const { container } = render(
        <GaugeFace currentPercent={0} startPercent={100} status="charging" />,
      );
      const arc = getChargingArc(container);
      const cmd = getArcCommand(arc);
      expect(cmd.sweep).toBe(0);
      expect(cmd.largeArc).toBe(1); // span = 100 → 270° > 180°
    });

    it("handles clamping of negative percentage", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={-10} status="charging" />,
      );
      const arc = getChargingArc(container);
      expect(arc).toBeTruthy();
      const cmd = getArcCommand(arc);
      // -10 should be clamped to 0, so start point should be at 0%
      const angle = (135 * Math.PI) / 180;
      const cx = 150;
      const cy = 150;
      const r = 300 * 0.42 * 0.85;
      const expectedX = cx + Math.cos(angle) * r;
      const expectedY = cy + Math.sin(angle) * r;
      expect(cmd.fromX).toBeCloseTo(expectedX, 1);
      expect(cmd.fromY).toBeCloseTo(expectedY, 1);
    });

    it("handles clamping of percentage > 100", () => {
      const { container } = render(
        <GaugeFace currentPercent={110} startPercent={30} status="charging" />,
      );
      const arc = getChargingArc(container);
      expect(arc).toBeTruthy();
      const cmd = getArcCommand(arc);
      // 110 should be clamped to 100, so end point should be at 100%
      const angle = (405 * Math.PI) / 180;
      const cx = 150;
      const cy = 150;
      const r = 300 * 0.42 * 0.85;
      const expectedX = cx + Math.cos(angle) * r;
      const expectedY = cy + Math.sin(angle) * r;
      expect(cmd.toX).toBeCloseTo(expectedX, 1);
      expect(cmd.toY).toBeCloseTo(expectedY, 1);
    });
  });

  describe("arc styling", () => {
    it("has correct stroke color", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      expect(arc?.getAttribute("stroke")).toBe("#22c55e");
    });

    it("has correct stroke width", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      expect(arc?.getAttribute("stroke-width")).toBe("3");
    });

    it("has round linecap", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      expect(arc?.getAttribute("stroke-linecap")).toBe("round");
    });

    it("has opacity 0.9", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      expect(arc?.getAttribute("opacity")).toBe("0.9");
    });

    it("has no fill", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="charging" />,
      );
      const arc = getChargingArc(container);
      expect(arc?.getAttribute("fill")).toBe("none");
    });
  });

  describe("background arc", () => {
    it("renders background arc", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="idle" />,
      );
      const paths = getPaths(container);
      const bgArc = paths.find(
        (p) => p.getAttribute("stroke") === "var(--color-gauge-track)",
      );
      expect(bgArc).toBeTruthy();
    });

    it("background arc spans full gauge (0 -> 100)", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="idle" />,
      );
      const paths = getPaths(container);
      const bgArc = paths.find(
        (p) => p.getAttribute("stroke") === "var(--color-gauge-track)",
      );
      const cmd = getArcCommand(bgArc);
      expect(cmd.largeArc).toBe(1); // span = 100 → 270° > 180°
      expect(cmd.sweep).toBe(1); // 100 >= 0
    });
  });

  describe("zone highlight arcs", () => {
    it("renders red zone arc (0-20%)", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="idle" />,
      );
      const paths = getPaths(container);
      const redArc = paths.find(
        (p) => p.getAttribute("stroke") === "rgba(239, 68, 68, 0.12)",
      );
      expect(redArc).toBeTruthy();
    });

    it("renders orange zone arc (80-100%)", () => {
      const { container } = render(
        <GaugeFace currentPercent={50} startPercent={20} status="idle" />,
      );
      const paths = getPaths(container);
      const orangeArc = paths.find(
        (p) => p.getAttribute("stroke") === "rgba(217, 119, 6, 0.12)",
      );
      expect(orangeArc).toBeTruthy();
    });
  });
});
