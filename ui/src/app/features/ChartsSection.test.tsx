import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

import ChartsSection from "./ChartsSection";

vi.mock("@/components/PowerChart", () => ({
  default: ({ vehicleId, shouldPoll, initialData }: any) => (
    <div data-testid="power-chart">
      PowerChart{vehicleId ? `-${vehicleId}` : ""}
      {shouldPoll ? "-polling" : ""}
      {initialData ? "-initial" : ""}
    </div>
  ),
}));

vi.mock("@/components/SocChart", () => ({
  default: ({ vehicleId, shouldPoll, initialData }: any) => (
    <div data-testid="soc-chart">
      SocChart{vehicleId ? `-${vehicleId}` : ""}
      {shouldPoll ? "-polling" : ""}
      {initialData ? "-initial" : ""}
    </div>
  ),
}));

vi.mock("@/components/CurrentChart", () => ({
  default: ({ vehicleId, shouldPoll, initialData }: any) => (
    <div data-testid="current-chart">
      CurrentChart{vehicleId ? `-${vehicleId}` : ""}
      {shouldPoll ? "-polling" : ""}
      {initialData ? "-initial" : ""}
    </div>
  ),
}));

vi.mock("@/components/CarbonIntensityChart", () => ({
  default: ({ vehicleId, shouldPoll, initialData }: any) => (
    <div data-testid="carbon-intensity-chart">
      CarbonIntensityChart{vehicleId ? `-${vehicleId}` : ""}
      {shouldPoll ? "-polling" : ""}
      {initialData ? "-initial" : ""}
    </div>
  ),
}));

vi.mock("@/components/ErrorBoundary", () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="error-boundary">{children}</div>
  ),
}));

describe("ChartsSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders all four charts", () => {
    render(<ChartsSection vehicleId="v1" shouldPoll={false} />);
    expect(screen.getByTestId("power-chart")).toBeInTheDocument();
    expect(screen.getByTestId("soc-chart")).toBeInTheDocument();
    expect(screen.getByTestId("current-chart")).toBeInTheDocument();
    expect(screen.getByTestId("carbon-intensity-chart")).toBeInTheDocument();
  });

  it("renders section labels", () => {
    render(<ChartsSection vehicleId="v1" shouldPoll={false} />);
    // Title and unit live in separate spans; match the outer label span by full textContent
    const byLabel = (text: string) =>
      screen.getAllByText(
        (_, el) => el?.tagName === "SPAN" && el?.textContent?.trim() === text,
      )[0];
    expect(byLabel("Power Draw (kW)")).toBeInTheDocument();
    expect(byLabel("State of Charge (%)")).toBeInTheDocument();
    expect(byLabel("Current Draw (A)")).toBeInTheDocument();
    expect(byLabel("Carbon Intensity (gCO₂/kWh)")).toBeInTheDocument();
  });

  it("passes vehicleId to PowerChart", () => {
    render(<ChartsSection vehicleId="v1" shouldPoll={false} />);
    expect(screen.getByText("PowerChart-v1")).toBeInTheDocument();
  });

  it("passes vehicleId to SocChart", () => {
    render(<ChartsSection vehicleId="v1" shouldPoll={false} />);
    expect(screen.getByText("SocChart-v1")).toBeInTheDocument();
  });

  it("passes vehicleId to CurrentChart", () => {
    render(<ChartsSection vehicleId="v1" shouldPoll={false} />);
    expect(screen.getByText("CurrentChart-v1")).toBeInTheDocument();
  });

  it("passes vehicleId to CarbonIntensityChart", () => {
    render(<ChartsSection vehicleId="v1" shouldPoll={false} />);
    expect(screen.getByText("CarbonIntensityChart-v1")).toBeInTheDocument();
  });

  it("passes shouldPoll=true to charts", () => {
    render(<ChartsSection vehicleId="v1" shouldPoll={true} />);
    expect(screen.getByText("PowerChart-v1-polling")).toBeInTheDocument();
    expect(screen.getByText("SocChart-v1-polling")).toBeInTheDocument();
    expect(screen.getByText("CurrentChart-v1-polling")).toBeInTheDocument();
    expect(
      screen.getByText("CarbonIntensityChart-v1-polling"),
    ).toBeInTheDocument();
  });

  it("passes shouldPoll=false to charts", () => {
    render(<ChartsSection vehicleId="v1" shouldPoll={false} />);
    expect(screen.getByText("PowerChart-v1")).toBeInTheDocument();
    expect(screen.getByText("SocChart-v1")).toBeInTheDocument();
  });

  it("handles null vehicleId", () => {
    render(<ChartsSection vehicleId={null} shouldPoll={false} />);
    expect(screen.getByText("PowerChart")).toBeInTheDocument();
    expect(screen.getByText("SocChart")).toBeInTheDocument();
    expect(screen.getByText("CurrentChart")).toBeInTheDocument();
    expect(screen.getByText("CarbonIntensityChart")).toBeInTheDocument();
  });

  it("handles undefined vehicleId", () => {
    render(<ChartsSection shouldPoll={false} />);
    expect(screen.getByText("PowerChart")).toBeInTheDocument();
    expect(screen.getByText("SocChart")).toBeInTheDocument();
  });

  it("wraps all charts in ErrorBoundary", () => {
    render(<ChartsSection vehicleId="v1" shouldPoll={false} />);
    const boundaries = screen.getAllByTestId("error-boundary");
    expect(boundaries).toHaveLength(4);
  });
});
