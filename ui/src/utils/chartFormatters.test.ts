import { describe, it, expect } from "vitest";

import { formatPower, formatSocPercent } from "./chartFormatters";

describe("formatPower", () => {
  it("returns kW when maxWatts >= 1000", () => {
    expect(formatPower(1500, 3000)).toBe("1.50 kW");
  });

  it("returns W when maxWatts < 1000", () => {
    expect(formatPower(500, 800)).toBe("500 W");
  });

  it("handles zero watts", () => {
    expect(formatPower(0, 3000)).toBe("0.00 kW");
  });
});

describe("formatSocPercent", () => {
  it("formats SOC value with 2 decimal places", () => {
    expect(formatSocPercent(75.5)).toBe("75.50%");
  });

  it("handles whole numbers", () => {
    expect(formatSocPercent(100)).toBe("100.00%");
  });
});
