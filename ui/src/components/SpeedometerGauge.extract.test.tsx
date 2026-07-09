import { useGaugeStore } from "@/stores/gaugeStore";
import { render } from "@testing-library/react";
import { describe, it, expect, beforeEach } from "vitest";

import { GaugeFace } from "./GaugeFace";
import { GaugeInfo } from "./GaugeInfo";
import { GaugeNeedle } from "./GaugeNeedle";
import { GaugeScale } from "./GaugeScale";

const VIEW_BOX = 300;
const CX = VIEW_BOX / 2;
const MAX_R = VIEW_BOX * 0.42;
const ARC_R = MAX_R * 0.85;

describe("GaugeFace", () => {
  beforeEach(() => {
    useGaugeStore.setState({ currentPercent: 25, targetPercent: 80 });
  });

  it("renders background circle", () => {
    const { container } = render(
      <GaugeFace currentPercent={25} startPercent={20} status="idle" />,
    );
    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
    const circles = svg?.querySelectorAll("circle");
    expect(circles?.length).toBe(3);
    expect(circles?.[0]?.getAttribute("fill")).toBe("var(--color-gauge-face)");
  });

  it("renders background arc track", () => {
    const { container } = render(
      <GaugeFace currentPercent={25} startPercent={20} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const paths = svg?.querySelectorAll("path");
    expect(paths?.length).toBeGreaterThanOrEqual(1);
    const bgArc = paths?.[0];
    expect(bgArc?.getAttribute("stroke")).toBe("var(--color-gauge-track)");
  });

  it("renders red and yellow warning segments", () => {
    const { container } = render(
      <GaugeFace currentPercent={25} startPercent={20} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const paths = svg?.querySelectorAll("path");
    const strokes = Array.from(paths ?? []).map((p) =>
      p.getAttribute("stroke"),
    );
    expect(strokes).toContain("rgba(239, 68, 68, 0.12)");
    expect(strokes).toContain("rgba(217, 119, 6, 0.12)");
  });

  it("renders green progress arc when charging", () => {
    const { container } = render(
      <GaugeFace currentPercent={50} startPercent={20} status="charging" />,
    );
    const svg = container.querySelector("svg");
    const paths = svg?.querySelectorAll("path");
    const greenArc = Array.from(paths ?? []).find(
      (p) => p.getAttribute("stroke") === "#22c55e",
    );
    expect(greenArc).toBeInTheDocument();
  });

  it("does not render green progress arc when idle", () => {
    const { container } = render(
      <GaugeFace currentPercent={50} startPercent={20} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const paths = svg?.querySelectorAll("path");
    const greenArc = Array.from(paths ?? []).find(
      (p) => p.getAttribute("stroke") === "#22c55e",
    );
    expect(greenArc).toBeUndefined();
  });

  it("uses correct viewBox", () => {
    const { container } = render(
      <GaugeFace currentPercent={25} startPercent={20} status="idle" />,
    );
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("viewBox")).toBe("0 0 300 300");
  });
});

describe("GaugeNeedle", () => {
  beforeEach(() => {
    useGaugeStore.setState({ currentPercent: 50, targetPercent: 80 });
  });

  it("renders needle line", () => {
    const { container } = render(<GaugeNeedle currentPercent={50} />);
    const svg = container.querySelector("svg");
    const lines = svg?.querySelectorAll("line");
    expect(lines?.length).toBe(1);
    expect(lines?.[0]?.getAttribute("stroke")).toBe("#ef4444");
  });

  it("renders needle tip circle", () => {
    const { container } = render(<GaugeNeedle currentPercent={25} />);
    const svg = container.querySelector("svg");
    const circles = svg?.querySelectorAll("circle");
    const tipCircle = Array.from(circles ?? []).find(
      (c) =>
        c.getAttribute("fill") === "#ef4444" && c.getAttribute("r") === "3",
    );
    expect(tipCircle).toBeInTheDocument();
  });

  it("renders center cap outer circle", () => {
    const { container } = render(<GaugeNeedle currentPercent={50} />);
    const svg = container.querySelector("svg");
    const circles = svg?.querySelectorAll("circle");
    const outerCap = Array.from(circles ?? []).find(
      (c) =>
        c.getAttribute("cx") === String(CX) &&
        c.getAttribute("cy") === String(CX) &&
        c.getAttribute("r") === "10",
    );
    expect(outerCap).toBeInTheDocument();
  });

  it("renders center cap inner circle", () => {
    const { container } = render(<GaugeNeedle currentPercent={50} />);
    const svg = container.querySelector("svg");
    const circles = svg?.querySelectorAll("circle");
    const innerCap = Array.from(circles ?? []).find(
      (c) =>
        c.getAttribute("cx") === String(CX) &&
        c.getAttribute("cy") === String(CX) &&
        c.getAttribute("r") === "4",
    );
    expect(innerCap).toBeInTheDocument();
  });

  it("needle originates from center", () => {
    const { container } = render(<GaugeNeedle currentPercent={50} />);
    const svg = container.querySelector("svg");
    const line = svg?.querySelector("line");
    expect(line?.getAttribute("x1")).toBe(String(CX));
    expect(line?.getAttribute("y1")).toBe(String(CX));
  });

  it("needle tip is offset from arc radius", () => {
    const { container } = render(<GaugeNeedle currentPercent={0} />);
    const svg = container.querySelector("svg");
    const line = svg?.querySelector("line");
    const x2 = parseFloat(line?.getAttribute("x2") ?? "0");
    const y2 = parseFloat(line?.getAttribute("y2") ?? "0");
    const dist = Math.sqrt((x2 - CX) ** 2 + (y2 - CX) ** 2);
    expect(dist).toBeCloseTo(ARC_R - 12, 1);
  });

  it("uses correct viewBox", () => {
    const { container } = render(<GaugeNeedle currentPercent={50} />);
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("viewBox")).toBe("0 0 300 300");
  });
});

describe("GaugeScale", () => {
  beforeEach(() => {
    useGaugeStore.setState({ currentPercent: 25, targetPercent: 80 });
  });

  it("renders 11 tick label text elements", () => {
    const { container } = render(
      <GaugeScale targetPercent={80} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const textElements = svg?.querySelectorAll("text");
    expect(textElements?.length).toBe(11);
  });

  it("renders tick marks for each label", () => {
    const { container } = render(
      <GaugeScale targetPercent={80} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const lines = svg?.querySelectorAll("line");
    const tickLines = Array.from(lines ?? []).filter(
      (l) => !l.getAttribute("stroke")?.includes("f97316"),
    );
    expect(tickLines.length).toBe(11);
  });

  it("major ticks have greater stroke width", () => {
    const { container } = render(
      <GaugeScale targetPercent={80} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const lines = svg?.querySelectorAll("line");
    const majorLines = Array.from(lines ?? []).filter(
      (l) =>
        l.getAttribute("stroke-width") === "2" &&
        !l.getAttribute("stroke")?.includes("f97316"),
    );
    expect(majorLines.length).toBe(6);
  });

  it("major labels have larger font size", () => {
    const { container } = render(
      <GaugeScale targetPercent={80} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const textElements = svg?.querySelectorAll("text");
    const majorTexts = Array.from(textElements ?? []).filter(
      (t) => t.getAttribute("font-size") === "9",
    );
    expect(majorTexts.length).toBe(6);
  });

  it("renders labels 0 through 100", () => {
    const { container } = render(
      <GaugeScale targetPercent={80} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const textElements = svg?.querySelectorAll("text");
    const labels = Array.from(textElements ?? []).map((t) =>
      t.textContent?.trim(),
    );
    expect(labels).toEqual([
      "0",
      "10",
      "20",
      "30",
      "40",
      "50",
      "60",
      "70",
      "80",
      "90",
      "100",
    ]);
  });

  it("renders target marker line and circle", () => {
    const { container } = render(
      <GaugeScale targetPercent={80} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const orangeCircles = svg?.querySelectorAll('circle[fill="#f97316"]');
    expect(orangeCircles?.length).toBe(1);
    const orangeLines = svg?.querySelectorAll('line[stroke="#f97316"]');
    expect(orangeLines?.length).toBe(1);
  });

  it("renders green start marker when charging", () => {
    const { container } = render(
      <GaugeScale targetPercent={80} status="charging" startPercent={20} />,
    );
    const svg = container.querySelector("svg");
    const greenCircles = svg?.querySelectorAll('circle[fill="#22c55e"]');
    expect(greenCircles?.length).toBe(1);
  });

  it("does not render green start marker when idle", () => {
    const { container } = render(
      <GaugeScale targetPercent={80} status="idle" />,
    );
    const svg = container.querySelector("svg");
    const greenCircles = svg?.querySelectorAll('circle[fill="#22c55e"]');
    expect(greenCircles?.length).toBe(0);
  });

  it("uses correct viewBox", () => {
    const { container } = render(
      <GaugeScale targetPercent={80} status="idle" />,
    );
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("viewBox")).toBe("0 0 300 300");
  });
});

describe("GaugeInfo", () => {
  beforeEach(() => {
    useGaugeStore.setState({ currentPercent: 25, targetPercent: 80 });
  });

  it("renders current SOC indicator", () => {
    const { container } = render(<GaugeInfo currentPercent={50} />);
    const svg = container.querySelector("svg");
    const circles = svg?.querySelectorAll("circle");
    const infoCircle = Array.from(circles ?? []).find(
      (c) => c.getAttribute("data-testid") === "gauge-info-current",
    );
    expect(infoCircle).toBeInTheDocument();
  });

  it("renders at correct position for given percentage", () => {
    const { container } = render(<GaugeInfo currentPercent={0} />);
    const svg = container.querySelector("svg");
    const circle = svg?.querySelector(
      'circle[data-testid="gauge-info-current"]',
    );
    expect(circle).toBeInTheDocument();
    const cx = parseFloat(circle?.getAttribute("cx") ?? "0");
    const cy = parseFloat(circle?.getAttribute("cy") ?? "0");
    const dist = Math.sqrt((cx - CX) ** 2 + (cy - CX) ** 2);
    expect(dist).toBeCloseTo(ARC_R, 1);
  });

  it("uses correct viewBox", () => {
    const { container } = render(<GaugeInfo currentPercent={50} />);
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("viewBox")).toBe("0 0 300 300");
  });
});
