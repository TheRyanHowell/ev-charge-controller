import { customRender, screen, waitFor, fireEvent, act } from "@/test-utils";
import { describe, it, expect, vi, beforeEach } from "vitest";

import VehicleDetailClient from "./VehicleDetailClient";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({
    push: vi.fn(),
  })),
}));

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    apiDelete: vi.fn(() => Promise.resolve({})),
    apiPatchNoContent: vi.fn(() => Promise.resolve()),
  };
});

vi.mock("@/components/CCVChart", () => ({
  default: () => <div data-testid="ccv-chart-mock" />,
}));

vi.mock("@/components/Dialog", () => ({
  default: function MockDialog({
    isOpen,
    onClose,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    children: React.ReactNode;
  }) {
    if (!isOpen) return null;
    return (
      <div data-testid="dialog" role="dialog">
        <button onClick={onClose}>Close Dialog</button>
        {children}
      </div>
    );
  },
}));

const { useRouter } = await import("next/navigation");
const { apiDelete, apiPatchNoContent } = await import("@/lib/api");

const mockVehicle = {
  id: "v1",
  userId: "u1",
  name: "My Tesla",
  modelId: "m1",
  modelName: "Tesla Model 3",
  capacityKwh: 75,
  startPercent: 20,
  targetPercent: 80,
  endPercent: 100,
  chargerOutputW: 3680,
  chargingEfficiency: 0.9,
  rangeMinMi: 0,
  rangeMaxMi: 358,
  packVoltageMaxV: 400,
  packCutoffCurrentMa: 5000,
  time0to100Min: 300,
  time0to80Min: 240,
  time20to80Min: 180,
  time20to100Min: 240,
  minSessionBatteryKwh: 0,
  maxSessionBatteryKwh: 0,
  notifyChargeComplete: true,
  notifyChargerOffline: true,
  notifyMaintenanceOffline: true,
  createdAt: "2024-01-01T00:00:00Z",
  updatedAt: "2024-01-01T00:00:00Z",
};

const mockStats = {
  totalSessions: 5,
  totalBatteryKwh: 150,
  avgSessionKwh: 30,
  totalCo2Grams: 45000,
  totalCostPence: 4138,
  avgCostPence: 828,
  avgCarbonGCo2PerKwh: 300,
  minSessionBatteryKwh: 20,
  maxSessionBatteryKwh: 40,
  dailyEnergy: [
    {
      date: "2024-01-01",
      batteryKwh: 10,
      sessionCount: 1,
      co2Grams: 3000,
      avgCarbonIntensityGCo2PerKwh: 300,
    },
    {
      date: "2024-01-02",
      batteryKwh: 20,
      sessionCount: 2,
      co2Grams: 6000,
      avgCarbonIntensityGCo2PerKwh: 300,
    },
  ],
};

const emptyStats = {
  totalSessions: 0,
  totalBatteryKwh: 0,
  avgSessionKwh: 0,
  totalCo2Grams: 0,
  avgCarbonGCo2PerKwh: undefined,
  minSessionBatteryKwh: 0,
  maxSessionBatteryKwh: 0,
  dailyEnergy: [],
};

describe("VehicleDetailClient", () => {
  const mockFetch = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockFetch);
    mockFetch.mockClear();
    (apiDelete as ReturnType<typeof vi.fn>).mockClear();
    (apiPatchNoContent as ReturnType<typeof vi.fn>).mockResolvedValue(
      undefined,
    );
    (useRouter as ReturnType<typeof vi.fn>).mockReturnValue({
      push: vi.fn(),
    });
  });

  it("renders vehicle name and model", () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    expect(screen.getByText("My Tesla (Tesla Model 3)")).toBeInTheDocument();
  });

  it("renders vehicle name without model when same as name", () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    const vehicleNoModel = { ...mockVehicle, modelName: "My Tesla" };
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={vehicleNoModel}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    expect(
      screen.getByRole("heading", { name: "My Tesla" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("(My Tesla)")).not.toBeInTheDocument();
  });

  it("shows no data message when no charging data", () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    expect(screen.getByText("No charging data yet")).toBeInTheDocument();
    expect(
      screen.getByText("Complete a charge session to see statistics"),
    ).toBeInTheDocument();
  });

  it("shows stats cards when data exists", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={mockStats}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("Total Energy")).toBeInTheDocument();
    });
    // Wall energy = battery energy / efficiency = 150 / 0.9 = 166.7
    expect(screen.getByText("166.7 kWh")).toBeInTheDocument();
    expect(screen.getByText("Sessions")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("shows time range buttons", () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    expect(screen.getByText("Week")).toBeInTheDocument();
    expect(screen.getByText("Month")).toBeInTheDocument();
    expect(screen.getByText("Year")).toBeInTheDocument();
    expect(screen.getByText("Lifetime")).toBeInTheDocument();
  });

  it("changes time range on button click", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const weekButton = screen.getByText("Week");
    expect(weekButton).toHaveClass("bg-blue-600");
    await act(async () => {
      fireEvent.click(screen.getByText("Month"));
    });
    expect(screen.getByText("Month")).toHaveClass("bg-blue-600");
    expect(weekButton).toHaveClass("bg-surface");
  });

  it("enters edit mode when edit button is clicked", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const editButton = screen.getByTitle("Edit name");
    await act(async () => {
      fireEvent.click(editButton);
    });
    expect(screen.getByDisplayValue("My Tesla")).toBeInTheDocument();
    expect(screen.getByTitle("Save")).toBeInTheDocument();
    expect(screen.getByTitle("Cancel")).toBeInTheDocument();
  });

  it("cancels editing when cancel button is clicked", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const editButton = screen.getByTitle("Edit name");
    await act(async () => {
      fireEvent.click(editButton);
    });
    await act(async () => {
      fireEvent.click(screen.getByTitle("Cancel"));
    });
    expect(screen.getByText("My Tesla (Tesla Model 3)")).toBeInTheDocument();
  });

  it("saves new name when save button is clicked", async () => {
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const editButton = screen.getByTitle("Edit name");
    await act(async () => {
      fireEvent.click(editButton);
    });
    await act(async () => {
      const input = screen.getByDisplayValue("My Tesla");
      fireEvent.change(input, { target: { value: "New Name" } });
      fireEvent.click(screen.getByTitle("Save"));
    });
    await waitFor(() => {
      expect(apiPatchNoContent).toHaveBeenCalledWith("/api/vehicles/v1", {
        name: "New Name",
      });
    });
  });

  it("shows error message when save fails", async () => {
    (apiPatchNoContent as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error("Name already taken"),
    );
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const editButton = screen.getByTitle("Edit name");
    await act(async () => {
      fireEvent.click(editButton);
    });
    await act(async () => {
      const input = screen.getByDisplayValue("My Tesla");
      fireEvent.change(input, { target: { value: "New Name" } });
      fireEvent.click(screen.getByTitle("Save"));
    });
    await waitFor(() => {
      expect(screen.getByText("Name already taken")).toBeInTheDocument();
    });
  });

  it("opens delete confirmation dialog", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const deleteButton = screen.getByTitle("Delete");
    await act(async () => {
      fireEvent.click(deleteButton);
    });
    expect(screen.getByTestId("dialog")).toBeInTheDocument();
    expect(screen.getByText("Delete vehicle?")).toBeInTheDocument();
  });

  it("closes delete dialog on cancel", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const deleteButton = screen.getByTitle("Delete");
    await act(async () => {
      fireEvent.click(deleteButton);
    });
    await act(async () => {
      fireEvent.click(screen.getByText("Cancel"));
    });
    expect(screen.queryByTestId("dialog")).not.toBeInTheDocument();
  });

  it("deletes vehicle and navigates away", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const deleteButton = screen.getByTitle("Delete");
    await act(async () => {
      fireEvent.click(deleteButton);
    });
    await act(async () => {
      fireEvent.click(screen.getByText("Delete"));
    });
    await waitFor(() => {
      expect(apiDelete).toHaveBeenCalledWith("/api/vehicles/v1");
    });
  });

  it("shows vehicle details section at the top regardless of session data", () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    expect(screen.getByText("Vehicle Details")).toBeInTheDocument();
    expect(screen.getByText("Tesla Model 3")).toBeInTheDocument();
    expect(screen.getByText("75 kWh")).toBeInTheDocument();
    expect(screen.getByText("3.7 kW")).toBeInTheDocument();
    expect(screen.getByText("90%")).toBeInTheDocument();
  });

  it("shows CC/CV charging profile chart", () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    expect(screen.getByText("CC/CV Charging Profile")).toBeInTheDocument();
    expect(screen.getByTestId("ccv-chart-mock")).toBeInTheDocument();
  });

  it("shows range details when available", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={mockStats}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("358 mi")).toBeInTheDocument();
    });
  });

  it("shows time formatting correctly", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={null}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("5h")).toBeInTheDocument(); // 300 min = 5h
      expect(screen.getAllByText("4h")).toHaveLength(2); // 240 min appears twice
      expect(screen.getByText("3h")).toBeInTheDocument(); // 180 min = 3h
    });
  });

  it("shows pack voltage and cutoff current details", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={mockStats}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("400 V")).toBeInTheDocument();
      expect(screen.getByText("5.00 A")).toBeInTheDocument();
    });
  });

  it("shows CO2 emissions with g/kWh when available", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={null}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getAllByText("CO₂ Emissions")).toHaveLength(2);
    });
    // 45000g = 45kg, with avg 300 g/kWh
    expect(screen.getByText("45.00 kg (300 g/kWh)")).toBeInTheDocument();
  });

  it("shows back link to vehicles page", () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const backLink = screen.getByRole("link", { name: "Back to vehicles" });
    expect(backLink).toHaveAttribute("href", "/vehicles");
  });

  it("disables save button when name is empty", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => emptyStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const editButton = screen.getByTitle("Edit name");
    await act(async () => {
      fireEvent.click(editButton);
    });
    await act(async () => {
      const input = screen.getByDisplayValue("My Tesla");
      fireEvent.change(input, { target: { value: "" } });
    });
    expect(screen.getByTitle("Save")).toBeDisabled();
  });

  it("clears error when user types in edit input", async () => {
    (apiPatchNoContent as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
      new Error("Error"),
    );
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={emptyStats}
        renderTimeMs={0}
      />,
    );
    const editButton = screen.getByTitle("Edit name");
    await act(async () => {
      fireEvent.click(editButton);
    });
    await act(async () => {
      const input = screen.getByDisplayValue("My Tesla");
      fireEvent.change(input, { target: { value: "New Name" } });
      fireEvent.click(screen.getByTitle("Save"));
    });
    await waitFor(() => {
      expect(screen.getByText("Error")).toBeInTheDocument();
    });
    await act(async () => {
      const input = screen.getByDisplayValue("New Name");
      fireEvent.change(input, { target: { value: "New Name " } });
    });
    expect(screen.queryByText("Error")).not.toBeInTheDocument();
  });

  // formatMinutes tests (tested via component rendering)
  it("formats 0 minutes as dash", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    const vehicle = { ...mockVehicle, time0to100Min: 0 };
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={vehicle}
        initialStats={null}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("-")).toBeInTheDocument();
    });
  });

  it("formats minutes only when under 1 hour", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    const vehicle = { ...mockVehicle, time0to100Min: 45 };
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={vehicle}
        initialStats={null}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("45m")).toBeInTheDocument();
    });
  });

  it("formats hours and minutes", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    const vehicle = { ...mockVehicle, time0to100Min: 90 };
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={vehicle}
        initialStats={null}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("1h 30m")).toBeInTheDocument();
    });
  });

  // formatCo2 tests (tested via component rendering)
  it("formats grams when under 1000", async () => {
    const stats500 = { ...mockStats, totalCo2Grams: 500 };
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => stats500,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={null}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getAllByText("CO₂ Emissions")).toHaveLength(2);
    });
    expect(screen.getByText("500 g (300 g/kWh)")).toBeInTheDocument();
  });

  it("formats kg when over 1000", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={null}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getAllByText("CO₂ Emissions")).toHaveLength(2);
    });
    expect(screen.getByText("45.00 kg (300 g/kWh)")).toBeInTheDocument();
  });

  it("shows total cost and avg cost per session stat cards", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={mockStats}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("Total Cost")).toBeInTheDocument();
      expect(screen.getByText("Avg Cost / Session")).toBeInTheDocument();
    });
    // Backend-provided frozen costs: 4138p → £41.38, 828p → £8.28.
    expect(screen.getByText("£41.38")).toBeInTheDocument();
    expect(screen.getByText("£8.28")).toBeInTheDocument();
  });

  it("shows min and max added range stat cards when rangeMaxMi > 0", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={mockVehicle}
        initialStats={mockStats}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("Min Added Range")).toBeInTheDocument();
      expect(screen.getByText("Max Added Range")).toBeInTheDocument();
    });
    // min: round(20/75 * 358) = round(95.47) = 95 mi
    // max: round(40/75 * 358) = round(190.93) = 191 mi
    expect(screen.getByText("95 mi")).toBeInTheDocument();
    expect(screen.getByText("191 mi")).toBeInTheDocument();
  });

  it("hides range cards when rangeMaxMi is 0", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockStats,
    });
    const vehicleNoRange = { ...mockVehicle, rangeMaxMi: 0 };
    customRender(
      <VehicleDetailClient
        vehicleId="v1"
        initialVehicle={vehicleNoRange}
        initialStats={mockStats}
        renderTimeMs={0}
      />,
    );
    await waitFor(() => {
      expect(screen.getByText("Total Energy")).toBeInTheDocument();
    });
    expect(screen.queryByText("Min Added Range")).not.toBeInTheDocument();
    expect(screen.queryByText("Max Added Range")).not.toBeInTheDocument();
  });
});
