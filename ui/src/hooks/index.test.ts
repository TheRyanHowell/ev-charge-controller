import { describe, it, expect } from "vitest";

import * as hooks from "./index";

describe("hooks barrel exports", () => {
  it("exports all hooks", () => {
    expect(hooks.useChargeSession).toBeDefined();
    expect(hooks.useHistory).toBeDefined();
    expect(hooks.useHistoryVehicles).toBeDefined();
    expect(hooks.useHistorySessions).toBeDefined();
    expect(hooks.useHistoryDelete).toBeDefined();
    expect(hooks.useSchedule).toBeDefined();
    expect(hooks.useVehicle).toBeDefined();
  });
});
