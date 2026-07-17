import { describe, it, expect } from "vitest";

import { hasBattery } from "./vehicle";

describe("hasBattery", () => {
  it("returns true for a vehicle with battery capacity", () => {
    expect(hasBattery({ capacityKwh: 2.026 })).toBe(true);
  });

  it("returns false for a battery-less (generic) vehicle", () => {
    expect(hasBattery({ capacityKwh: 0 })).toBe(false);
  });
});
