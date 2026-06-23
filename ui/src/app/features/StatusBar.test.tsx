import { useGaugeStore } from "@/stores/gaugeStore";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import StatusBar from "./StatusBar";

const testVehicle = {
  id: "v1",
  name: "Test Vehicle",
  capacityKwh: 5.46,
  chargerOutputW: 600,
  chargingEfficiency: 0.8,
  rangeMinMi: 0,
  rangeMaxMi: 0,
};

beforeEach(() => {
  useGaugeStore.setState({ currentPercent: 50, targetPercent: 80 });
});

describe("StatusBar", () => {
  it("renders no vehicle message when no vehicle assigned", () => {
    render(<StatusBar tempError={null} selectedVehicle={null} />);
    expect(
      screen.getByText(/No vehicle assigned to this plug/i),
    ).toBeInTheDocument();
  });

  it("renders vehicle name when selected", () => {
    render(<StatusBar tempError={null} selectedVehicle={testVehicle} />);
    expect(screen.getByText("Test Vehicle")).toBeInTheDocument();
  });

  it("renders vehicle capacity", () => {
    render(<StatusBar tempError={null} selectedVehicle={testVehicle} />);
    expect(screen.getByText("5.46 kWh")).toBeInTheDocument();
  });

  it("renders charger output", () => {
    render(<StatusBar tempError={null} selectedVehicle={testVehicle} />);
    expect(screen.getByText("600 W")).toBeInTheDocument();
  });

  it("renders temp error with role=alert", () => {
    render(
      <StatusBar
        tempError="Something went wrong"
        selectedVehicle={testVehicle}
      />,
    );
    const alert = screen.getByRole("alert");
    expect(alert).toBeInTheDocument();
    expect(alert).toHaveTextContent("Something went wrong");
  });

  it("does not render temp error when null", () => {
    render(<StatusBar tempError={null} selectedVehicle={testVehicle} />);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("does not show no vehicle message when vehicle is selected", () => {
    render(<StatusBar tempError={null} selectedVehicle={testVehicle} />);
    expect(
      screen.queryByText(/No vehicle assigned to this plug/i),
    ).not.toBeInTheDocument();
  });

  it("formats charger output correctly for high wattage", () => {
    const highWattVehicle = { ...testVehicle, chargerOutputW: 2400 };
    render(<StatusBar tempError={null} selectedVehicle={highWattVehicle} />);
    expect(screen.getByText("2.40 kW")).toBeInTheDocument();
  });
});
