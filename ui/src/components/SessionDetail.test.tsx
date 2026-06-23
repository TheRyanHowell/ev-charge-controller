import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import SessionDetail from "./SessionDetail";

vi.mock("./PowerChart", () => ({
  default: vi.fn(({ sessionId }: { sessionId: string }) => (
    <div data-testid="mock-power-chart" data-session-id={sessionId} />
  )),
}));

vi.mock("./SocChart", () => ({
  default: vi.fn(({ sessionId }: { sessionId: string }) => (
    <div data-testid="mock-soc-chart" data-session-id={sessionId} />
  )),
}));

vi.mock("./CurrentChart", () => ({
  default: vi.fn(({ sessionId }: { sessionId: string }) => (
    <div data-testid="mock-current-chart" data-session-id={sessionId} />
  )),
}));

vi.mock("./CarbonIntensityChart", () => ({
  default: vi.fn(({ sessionId }: { sessionId: string }) => (
    <div
      data-testid="mock-carbon-intensity-chart"
      data-session-id={sessionId}
    />
  )),
}));

describe("SessionDetail", () => {
  it("renders Power Draw heading", () => {
    render(<SessionDetail sessionId="test-session-1" />);
    expect(screen.getByText("Power Draw (kW)")).toBeInTheDocument();
  });

  it("renders State of Charge heading", () => {
    render(<SessionDetail sessionId="test-session-1" />);
    expect(screen.getByText("State of Charge (%)")).toBeInTheDocument();
  });

  it("renders Current Draw heading", () => {
    render(<SessionDetail sessionId="test-session-1" />);
    expect(screen.getByText("Current Draw (A)")).toBeInTheDocument();
  });

  it("renders Carbon Intensity heading", () => {
    render(<SessionDetail sessionId="test-session-1" />);
    expect(screen.getByText("Carbon Intensity (gCO₂/kWh)")).toBeInTheDocument();
  });

  it("passes sessionId to PowerChart", () => {
    render(<SessionDetail sessionId="abc123" />);
    expect(screen.getByTestId("mock-power-chart")).toHaveAttribute(
      "data-session-id",
      "abc123",
    );
  });

  it("passes sessionId to SocChart", () => {
    render(<SessionDetail sessionId="abc123" />);
    expect(screen.getByTestId("mock-soc-chart")).toHaveAttribute(
      "data-session-id",
      "abc123",
    );
  });

  it("passes sessionId to CurrentChart", () => {
    render(<SessionDetail sessionId="abc123" />);
    expect(screen.getByTestId("mock-current-chart")).toHaveAttribute(
      "data-session-id",
      "abc123",
    );
  });

  it("passes sessionId to CarbonIntensityChart", () => {
    render(<SessionDetail sessionId="abc123" />);
    expect(screen.getByTestId("mock-carbon-intensity-chart")).toHaveAttribute(
      "data-session-id",
      "abc123",
    );
  });

  it("renders space-y-2 layout", () => {
    const { container } = render(<SessionDetail sessionId="test-session-1" />);
    const div = container.firstChild as HTMLElement;
    expect(div.className).toContain("space-y-2");
  });
});
