import { render, screen } from "@/test-utils";
import { describe, it, expect } from "vitest";

import { renderCurrentTooltip } from "./CurrentTooltip";

describe("renderCurrentTooltip", () => {
  it("renders current value with 2 decimal places", () => {
    render(renderCurrentTooltip(10.5, "10:30"));
    expect(screen.getByText("10.50 A")).toBeInTheDocument();
  });

  it("renders timestamp in tooltip", () => {
    render(renderCurrentTooltip(5.25, "14:15"));
    expect(screen.getByText("14:15")).toBeInTheDocument();
  });

  it("renders value with zero current", () => {
    render(renderCurrentTooltip(0, "08:00"));
    expect(screen.getByText("0.00 A")).toBeInTheDocument();
  });

  it("renders value with high current", () => {
    render(renderCurrentTooltip(32.123, "23:59"));
    expect(screen.getByText("32.12 A")).toBeInTheDocument();
  });

  it("renders value with single decimal rounded to two places", () => {
    render(renderCurrentTooltip(7.1, "12:00"));
    expect(screen.getByText("7.10 A")).toBeInTheDocument();
  });

  it("renders tooltip container with correct styling", () => {
    const { container } = render(renderCurrentTooltip(10, "10:00"));
    const tooltip = container.querySelector("div");
    expect(tooltip).toBeInTheDocument();
    expect(tooltip).toHaveClass("bg-gray-800");
    expect(tooltip).toHaveClass("text-white");
    expect(tooltip).toHaveClass("text-xs");
    expect(tooltip).toHaveClass("rounded");
  });

  it("renders value with blue-400 color class", () => {
    const { container } = render(renderCurrentTooltip(10, "10:00"));
    const valueSpan = container.querySelector(".text-blue-400");
    expect(valueSpan).toBeInTheDocument();
    expect(valueSpan).toHaveClass("font-semibold");
  });

  it("renders timestamp with gray-400 color class", () => {
    const { container } = render(renderCurrentTooltip(10, "10:00"));
    const timeSpan = container.querySelector(".text-gray-400");
    expect(timeSpan).toBeInTheDocument();
  });

  it("renders separator between value and timestamp", () => {
    render(renderCurrentTooltip(10, "10:00"));
    expect(screen.getByText(/\u00b7/)).toBeInTheDocument();
  });

  it("renders with negative current (edge case)", () => {
    render(renderCurrentTooltip(-2.5, "10:00"));
    expect(screen.getByText("-2.50 A")).toBeInTheDocument();
  });

  it("renders with very small current value", () => {
    render(renderCurrentTooltip(0.001, "10:00"));
    expect(screen.getByText("0.00 A")).toBeInTheDocument();
  });
});
