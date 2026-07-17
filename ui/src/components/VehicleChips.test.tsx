import { createPlug, createVehicle } from "@/test/fixtures";
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import VehicleChips from "./VehicleChips";

const vehicles = [
  createVehicle({ id: "v1", name: "My RM1" }),
  createVehicle({ id: "v2", name: "My RM1S" }),
];

const chargingPlugV1 = createPlug({
  id: "p1",
  vehicleId: "v1",
  type: "charging",
  online: true,
});
const chargingPlugV2 = createPlug({
  id: "p2",
  vehicleId: "v2",
  type: "charging",
  online: false,
});

describe("VehicleChips", () => {
  it("renders a chip for each vehicle", () => {
    render(
      <VehicleChips
        vehicles={vehicles}
        plugs={[chargingPlugV1, chargingPlugV2]}
        selectedVehicleId={null}
        onSelect={vi.fn()}
      />,
    );
    expect(screen.getByText("My RM1")).toBeInTheDocument();
    expect(screen.getByText("My RM1S")).toBeInTheDocument();
  });

  it("marks the selected vehicle chip as pressed", () => {
    render(
      <VehicleChips
        vehicles={vehicles}
        plugs={[chargingPlugV1, chargingPlugV2]}
        selectedVehicleId="v1"
        onSelect={vi.fn()}
      />,
    );
    // Button accessible name includes the online-dot aria-label: "Online My RM1"
    const chip = screen.getByRole("button", { name: "Online My RM1" });
    expect(chip).toHaveAttribute("aria-pressed", "true");
    const chip2 = screen.getByRole("button", { name: "Offline My RM1S" });
    expect(chip2).toHaveAttribute("aria-pressed", "false");
  });

  it("shows green dot for vehicle with online charging plug", () => {
    render(
      <VehicleChips
        vehicles={[createVehicle({ id: "v1", name: "My RM1" })]}
        plugs={[chargingPlugV1]}
        selectedVehicleId={null}
        onSelect={vi.fn()}
      />,
    );
    const onlineDot = screen.getByLabelText("Online");
    expect(onlineDot).toBeInTheDocument();
  });

  it("shows gray dot for vehicle with offline charging plug", () => {
    render(
      <VehicleChips
        vehicles={[createVehicle({ id: "v2", name: "My RM1S" })]}
        plugs={[chargingPlugV2]}
        selectedVehicleId={null}
        onSelect={vi.fn()}
      />,
    );
    const offlineDot = screen.getByLabelText("Offline");
    expect(offlineDot).toBeInTheDocument();
  });

  it("calls onSelect with the vehicle id when chip is clicked", () => {
    const onSelect = vi.fn();
    render(
      <VehicleChips
        vehicles={vehicles}
        plugs={[chargingPlugV1, chargingPlugV2]}
        selectedVehicleId={null}
        onSelect={onSelect}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Offline My RM1S" }));
    expect(onSelect).toHaveBeenCalledWith("v2");
  });

  it("does not render an add button", () => {
    render(
      <VehicleChips
        vehicles={vehicles}
        plugs={[]}
        selectedVehicleId={null}
        onSelect={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("button", { name: "Add vehicle" }),
    ).not.toBeInTheDocument();
  });

  it("returns null when no vehicles", () => {
    const { container } = render(
      <VehicleChips
        vehicles={[]}
        plugs={[]}
        selectedVehicleId={null}
        onSelect={vi.fn()}
      />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("shows gray dot when vehicle has no charging plug", () => {
    render(
      <VehicleChips
        vehicles={[createVehicle({ id: "v1", name: "Unassigned" })]}
        plugs={[]}
        selectedVehicleId={null}
        onSelect={vi.fn()}
      />,
    );
    expect(screen.getByLabelText("Offline")).toBeInTheDocument();
  });

  it("ignores maintenance plug for online status", () => {
    const maintenancePlug = createPlug({
      id: "m1",
      vehicleId: "v1",
      type: "maintenance",
      online: true,
    });
    render(
      <VehicleChips
        vehicles={[createVehicle({ id: "v1", name: "My RM1" })]}
        plugs={[maintenancePlug]}
        selectedVehicleId={null}
        onSelect={vi.fn()}
      />,
    );
    // Maintenance plug does not count - online dot should be "Offline"
    expect(screen.getByLabelText("Offline")).toBeInTheDocument();
  });

  it("uses maintenance plug online status for battery-less vehicles", () => {
    const maintenancePlug = createPlug({
      id: "m1",
      vehicleId: "v1",
      type: "maintenance",
      online: true,
    });
    render(
      <VehicleChips
        vehicles={[
          createVehicle({
            id: "v1",
            name: "My Petrol Bike",
            capacityKwh: 0,
            chargerOutputW: 0,
          }),
        ]}
        plugs={[maintenancePlug]}
        selectedVehicleId={null}
        onSelect={vi.fn()}
      />,
    );
    // A generic vehicle can only have a maintenance plug, so its online
    // state is the vehicle's online state.
    expect(screen.getByLabelText("Online")).toBeInTheDocument();
  });
});
