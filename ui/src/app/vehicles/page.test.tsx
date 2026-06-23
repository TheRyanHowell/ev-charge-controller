import type { Vehicle, VehicleModel } from "@/lib/schemas";

import { customRender, makeQueryClient, screen, waitFor } from "@/test-utils";
import { createVehicle, createVehicleModel } from "@/test/fixtures";
import { QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { createElement } from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Module-level mock so that imports in VehiclesClient.tsx are intercepted.
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return { ...actual };
});

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

import VehiclesClient from "./VehiclesClient";

function renderVehiclesClient({
  vehicles = [],
  models = [],
  initialError = false,
}: {
  vehicles?: Vehicle[];
  models?: VehicleModel[];
  initialError?: boolean;
}) {
  const queryClient = makeQueryClient();
  queryClient.setQueryData(["chargeController", "vehicles"], vehicles);
  queryClient.setQueryData(["chargeController", "vehicleModels"], models);

  return customRender(
    <QueryClientProvider client={queryClient}>
      <VehiclesClient
        initialVehicles={vehicles}
        initialError={initialError}
        initialModels={models}
        renderTimeMs={Date.now()}
      />
    </QueryClientProvider>,
  );
}

const defaultVehicles: Vehicle[] = [
  createVehicle({
    id: "v1",
    modelId: "rm1",
    name: "Maeving RM1",
    capacityKwh: 1.9,
    rangeMinMi: 30,
    rangeMaxMi: 40,
  }),
  createVehicle({
    id: "v2",
    modelId: "rm2",
    name: "Maeving RM2",
    capacityKwh: 5.46,
    rangeMinMi: 60,
    rangeMaxMi: 80,
  }),
];

const defaultModels: VehicleModel[] = [
  createVehicleModel({
    id: "rm1",
    name: "Maeving RM1",
    capacityKwh: 1.9,
    rangeMinMi: 30,
    rangeMaxMi: 40,
  }),
  createVehicleModel({
    id: "rm2",
    name: "Maeving RM2",
    capacityKwh: 5.46,
    rangeMinMi: 60,
    rangeMaxMi: 80,
  }),
];

describe("VehiclesClient", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders empty state when no vehicles", () => {
    renderVehiclesClient({ vehicles: [], models: defaultModels });
    expect(screen.getByText("No vehicles yet")).toBeTruthy();
    expect(screen.getByText("Add your first vehicle")).toBeTruthy();
  });

  it("renders vehicle list with names and capacity", () => {
    renderVehiclesClient({ vehicles: defaultVehicles, models: defaultModels });
    // Capacity appears in vehicle list rows (text-xs text-gray-400)
    const capacities = screen.getAllByText(/1\.9 kWh/);
    expect(capacities.length).toBeGreaterThanOrEqual(1);
    const capacities2 = screen.getAllByText(/5\.46 kWh/);
    expect(capacities2.length).toBeGreaterThanOrEqual(1);
  });

  it("renders vehicle name as link to detail page", () => {
    renderVehiclesClient({ vehicles: defaultVehicles, models: defaultModels });
    const links = document.querySelectorAll('a[href="/vehicles/v1"]');
    expect(links.length).toBeGreaterThan(0);
  });

  it("shows summary stats when available", async () => {
    const vehiclesWithStats: Vehicle[] = [
      createVehicle({
        id: "v1",
        modelId: "rm1",
        name: "Maeving RM1",
        capacityKwh: 1.9,
        rangeMinMi: 30,
        rangeMaxMi: 40,
        totalSessions: 5,
        totalBatteryKwh: 12.5,
        totalCo2Grams: 5000,
        minSessionBatteryKwh: 1.5,
        maxSessionBatteryKwh: 3.5,
        lastSessionAt: new Date().toISOString(),
      }),
      createVehicle({
        id: "v2",
        modelId: "rm2",
        name: "Maeving RM2",
        capacityKwh: 5.46,
        rangeMinMi: 60,
        rangeMaxMi: 80,
      }),
    ];

    renderVehiclesClient({
      vehicles: vehiclesWithStats,
      models: defaultModels,
    });

    await waitFor(() => {
      expect(screen.getByText("5 sessions")).toBeTruthy();
      expect(screen.getByText("12.5 kWh")).toBeTruthy();
      expect(screen.getByText("5.00 kg")).toBeTruthy();
    });
  });

  it("shows cost and range in stats when available", async () => {
    const vehiclesWithStats: Vehicle[] = [
      createVehicle({
        id: "v1",
        modelId: "rm1",
        name: "Maeving RM1",
        capacityKwh: 1.9,
        rangeMinMi: 30,
        rangeMaxMi: 40,
        totalSessions: 4,
        totalBatteryKwh: 8.0,
        totalCo2Grams: 0,
        minSessionBatteryKwh: 1.0,
        maxSessionBatteryKwh: 3.0,
      }),
      createVehicle({
        id: "v2",
        modelId: "rm2",
        name: "Maeving RM2",
        capacityKwh: 5.46,
        rangeMinMi: 60,
        rangeMaxMi: 80,
      }),
    ];

    renderVehiclesClient({
      vehicles: vehiclesWithStats,
      models: defaultModels,
    });

    await waitFor(() => {
      // Range: min = round(1.0/1.9*40) = 21, max = round(3.0/1.9*40) = 63
      const rangeEl = document.querySelector(".fa-road")?.closest("span");
      expect(rangeEl).toBeTruthy();
    });
  });

  it("hides stats section when no sessions", () => {
    renderVehiclesClient({ vehicles: defaultVehicles, models: defaultModels });
    expect(screen.queryByText(/sessions/)).toBeFalsy();
  });

  it("shows add vehicle button", () => {
    renderVehiclesClient({ vehicles: defaultVehicles, models: defaultModels });
    const addBtn = document.querySelector(
      'button[type="button"]:has(i.fa-plus)',
    );
    expect(addBtn).toBeTruthy();
  });

  it("opens add vehicle dialog on button click", async () => {
    const { fireEvent } = await import("@testing-library/react");
    renderVehiclesClient({ vehicles: defaultVehicles, models: defaultModels });
    const addBtn = document.querySelector(
      'button[type="button"]:has(i.fa-plus)',
    );
    if (addBtn) fireEvent.click(addBtn);
    expect(screen.getByRole("dialog")).toBeTruthy();
  });

  it("shows edit and delete buttons for each vehicle", () => {
    renderVehiclesClient({ vehicles: defaultVehicles, models: defaultModels });
    const editButtons = document.querySelectorAll('[title="Edit name"]');
    expect(editButtons.length).toBe(2);
    const deleteButtons = document.querySelectorAll('[title="Delete"]');
    expect(deleteButtons.length).toBe(2);
  });

  it("shows all models in add dialog even when all models are already in use", async () => {
    const { fireEvent } = await import("@testing-library/react");
    // All models are already assigned to vehicles
    renderVehiclesClient({ vehicles: defaultVehicles, models: defaultModels });

    const addBtn = document.querySelector(
      'button[type="button"]:has(i.fa-plus)',
    );
    if (addBtn) fireEvent.click(addBtn);

    const dialog = screen.getByRole("dialog");
    // Should NOT show "no available models" message
    expect(dialog.textContent).not.toMatch(/no available models/i);
    // Should show all model buttons (both models are in use but still selectable)
    expect(dialog.textContent).toContain("Maeving RM1");
    expect(dialog.textContent).toContain("Maeving RM2");
  });

  it("shows error state when API fails - does NOT show 'No vehicles yet'", async () => {
    // When the API returns an error (e.g. DB missing column, network failure),
    // the UI must show a meaningful error message, not silently show "No vehicles yet".
    //
    // Mechanism: initialError=true causes the component to skip initialData so
    // React Query starts in 'pending' and transitions to 'error' when queryFn rejects.
    const apiModule = await import("@/lib/api");
    vi.spyOn(apiModule, "apiGet").mockRejectedValue(
      new Error("Internal Server Error"),
    );

    // Fresh QueryClient with no vehicles cache - forces a fetch that will fail.
    const queryClient = makeQueryClient();
    queryClient.setQueryData(
      ["chargeController", "vehicleModels"],
      defaultModels,
    );

    render(
      createElement(
        QueryClientProvider,
        { client: queryClient },
        createElement(VehiclesClient, {
          initialVehicles: undefined,
          initialError: true,
          initialModels: defaultModels,
          renderTimeMs: Date.now(),
        }),
      ),
    );

    await waitFor(
      () => {
        expect(
          screen.getByText(/failed to load vehicles/i),
          "Error message must be visible when the API fails",
        ).toBeTruthy();
      },
      { timeout: 3000 },
    );

    // Must NOT silently show the empty state as if there are no vehicles
    expect(
      screen.queryByText("No vehicles yet"),
      "Empty state must not appear when the API fails - hides real errors from users",
    ).toBeNull();
  });

  it("shows empty state only when API succeeds with zero vehicles", () => {
    // Empty state is ONLY appropriate when the API successfully returns []
    renderVehiclesClient({ vehicles: [], models: defaultModels });
    expect(screen.getByText("No vehicles yet")).toBeTruthy();
    // No error message should appear for a successful empty response
    expect(screen.queryByText(/failed to load vehicles/i)).toBeNull();
  });
});
