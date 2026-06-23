import { customRender, screen } from "@/test-utils";
import { createVehicle } from "@/test/fixtures";
import { describe, it, expect, vi, beforeEach } from "vitest";

import CCVChart from "./CCVChart";

const mockVehicle = createVehicle({
  id: "v1",
  name: "Test Vehicle",
  capacityKwh: 2,
  chargerOutputW: 1500,
  chargingEfficiency: 0.85,
  time20to80Min: 60,
  time0to80Min: 80,
  time0to100Min: 110,
  time20to100Min: 90,
  packVoltageMaxV: 58.8,
  packCutoffCurrentMa: 600,
  rangeMinMi: 50,
  rangeMaxMi: 100,
});

describe("CCVChart", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
    });
  });

  it("renders chart when vehicle data is provided", () => {
    customRender(<CCVChart vehicle={mockVehicle} />);
    expect(
      document.querySelector(".recharts-responsive-container"),
    ).toBeInTheDocument();
  });

  it("renders with aria label for accessibility", () => {
    customRender(<CCVChart vehicle={mockVehicle} />);
    const chartImg = document.querySelector('[role="img"]');
    expect(chartImg).toBeInTheDocument();
    expect(chartImg).toHaveAttribute("aria-label", "CC/CV charging profile");
  });

  it("renders with custom messages for empty state", () => {
    const minimalVehicle = {
      id: "v2",
      name: "Minimal Vehicle",
      capacityKwh: 0,
      chargerOutputW: 0,
      chargingEfficiency: 0.85,
      rangeMinMi: 50,
      rangeMaxMi: 100,
    };
    customRender(<CCVChart vehicle={minimalVehicle} />);
    expect(screen.getByText(/no profile data/i)).toBeInTheDocument();
  });

  it("does not call fetch since data is computed statically", () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({ ok: true, json: async () => [] }),
    );
    vi.stubGlobal("fetch", mockFetch);
    customRender(<CCVChart vehicle={mockVehicle} />);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("renders chart container with correct structure", () => {
    customRender(<CCVChart vehicle={mockVehicle} />);
    const container = document.querySelector(
      ".mt-1.sm\\:mt-2.rounded-lg.overflow-hidden",
    );
    expect(container).toBeInTheDocument();
  });

  it("renders with vehicle having only minimal required fields", () => {
    const minimalVehicle = {
      id: "v3",
      name: "Basic Vehicle",
      capacityKwh: 1.5,
      chargerOutputW: 1000,
      chargingEfficiency: 0.8,
      rangeMinMi: 30,
      rangeMaxMi: 60,
    };
    customRender(<CCVChart vehicle={minimalVehicle} />);
    expect(
      document.querySelector(".recharts-responsive-container"),
    ).toBeInTheDocument();
  });

  it("renders with all optional timing fields provided", () => {
    const fullVehicle = {
      ...mockVehicle,
      time0to100Min: 120,
      time0to80Min: 85,
      time20to80Min: 55,
      time20to100Min: 80,
      packVoltageMaxV: 60,
      packCutoffCurrentMa: 700,
    };
    customRender(<CCVChart vehicle={fullVehicle} />);
    expect(
      document.querySelector(".recharts-responsive-container"),
    ).toBeInTheDocument();
  });

  it("renders with vehicle having zero capacity", () => {
    const zeroCapacityVehicle = {
      id: "v5",
      name: "Zero Vehicle",
      capacityKwh: 0,
      chargerOutputW: 0,
      chargingEfficiency: 0.85,
      rangeMinMi: 50,
      rangeMaxMi: 100,
    };
    customRender(<CCVChart vehicle={zeroCapacityVehicle} />);
    expect(screen.getByText(/no profile data/i)).toBeInTheDocument();
  });

  it("renders with different vehicle ids producing independent charts", () => {
    const vehicle2 = {
      ...mockVehicle,
      id: "v4",
      chargerOutputW: 2000,
    };
    const { unmount } = customRender(<CCVChart vehicle={mockVehicle} />);
    expect(
      document.querySelector(".recharts-responsive-container"),
    ).toBeInTheDocument();
    unmount();

    customRender(<CCVChart vehicle={vehicle2} />);
    expect(
      document.querySelector(".recharts-responsive-container"),
    ).toBeInTheDocument();
  });
});
