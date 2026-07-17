import { createVehicle, createVehicleModel } from "@/test/fixtures";
import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import VehicleSelector from "./VehicleSelector";

const batteryVehicle = createVehicle({
  id: "v1",
  name: "My RM1",
  modelId: "rm1",
});
const genericVehicle = createVehicle({
  id: "v2",
  name: "My Petrol Bike",
  modelId: "generic",
  modelName: "Generic Vehicle",
  capacityKwh: 0,
  chargerOutputW: 0,
});
const batteryModel = createVehicleModel({
  id: "rm1s",
  name: "Maeving RM1S",
  capacityKwh: 5.46,
});
const genericModel = createVehicleModel({
  id: "generic",
  name: "Generic Vehicle",
  capacityKwh: 0,
  chargerOutputW: 0,
});

describe("VehicleSelector", () => {
  it("lists battery vehicles with their capacity", () => {
    render(
      <VehicleSelector
        label="Vehicle *"
        vehicles={[batteryVehicle]}
        models={[batteryModel]}
        selectedVehicleId={null}
        onSelectVehicle={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("option", { name: /My RM1 .* 2\.026 kWh/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: /Maeving RM1S · 5.46 kWh/ }),
    ).toBeInTheDocument();
  });

  it("excludes battery-less vehicles and models - a charging plug cannot serve them", () => {
    render(
      <VehicleSelector
        label="Vehicle *"
        vehicles={[batteryVehicle, genericVehicle]}
        models={[batteryModel, genericModel]}
        selectedVehicleId={null}
        onSelectVehicle={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("option", { name: /My Petrol Bike/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: /Generic Vehicle/ }),
    ).not.toBeInTheDocument();
  });
});
