import { render, screen } from "@/test-utils";
import { describe, it, expect } from "vitest";

import { renderCarbonIntensityTooltip } from "./CarbonIntensityTooltip";

describe("renderCarbonIntensityTooltip", () => {
  it("renders carbon intensity value rounded to integer", () => {
    const { container } = render(renderCarbonIntensityTooltip(350.7, "10:30"));
    const valueSpan = container.querySelector(".text-lime-400");
    expect(valueSpan).toHaveTextContent("351 gCO\u2082/kWh");
  });

  it("renders timestamp in tooltip", () => {
    render(renderCarbonIntensityTooltip(200, "14:15"));
    expect(screen.getByText("14:15")).toBeInTheDocument();
  });

  it("renders value with zero intensity", () => {
    const { container } = render(renderCarbonIntensityTooltip(0, "08:00"));
    const valueSpan = container.querySelector(".text-lime-400");
    expect(valueSpan).toHaveTextContent("0 gCO\u2082/kWh");
  });

  it("renders value with high intensity", () => {
    const { container } = render(renderCarbonIntensityTooltip(999.4, "23:59"));
    const valueSpan = container.querySelector(".text-lime-400");
    expect(valueSpan).toHaveTextContent("999 gCO\u2082/kWh");
  });

  it("renders decimal value rounded correctly", () => {
    const { container } = render(renderCarbonIntensityTooltip(450.5, "12:00"));
    const valueSpan = container.querySelector(".text-lime-400");
    expect(valueSpan).toHaveTextContent("451 gCO\u2082/kWh");
  });

  it("renders tooltip container with correct styling", () => {
    const { container } = render(renderCarbonIntensityTooltip(300, "10:00"));
    const tooltip = container.querySelector("div");
    expect(tooltip).toBeInTheDocument();
    expect(tooltip).toHaveClass("bg-gray-800");
    expect(tooltip).toHaveClass("text-white");
    expect(tooltip).toHaveClass("text-xs");
    expect(tooltip).toHaveClass("rounded");
  });

  it("renders value with lime-400 color class", () => {
    const { container } = render(renderCarbonIntensityTooltip(300, "10:00"));
    const valueSpan = container.querySelector(".text-lime-400");
    expect(valueSpan).toBeInTheDocument();
    expect(valueSpan).toHaveClass("font-semibold");
  });

  it("renders timestamp with gray-400 color class", () => {
    const { container } = render(renderCarbonIntensityTooltip(300, "10:00"));
    const timeSpan = container.querySelector(".text-gray-400");
    expect(timeSpan).toBeInTheDocument();
  });

  it("renders separator between value and timestamp", () => {
    render(renderCarbonIntensityTooltip(300, "10:00"));
    expect(screen.getByText(/\u00b7/)).toBeInTheDocument();
  });

  it("renders with negative value (edge case)", () => {
    const { container } = render(renderCarbonIntensityTooltip(-10, "10:00"));
    const valueSpan = container.querySelector(".text-lime-400");
    expect(valueSpan).toHaveTextContent("-10 gCO\u2082/kWh");
  });
});
