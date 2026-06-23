import {
  integrationETA,
  powerAtSOC,
  computeCVTimeMin,
  computeTime0to20Min,
  buildStaticCurve,
  effectiveCapacityKwh,
  dynamicEffectiveCapacityKwh,
  hasCurveData,
  calculateETA,
} from "@/utils/eta";
import { describe, it, expect } from "vitest";

describe("integrationETA", () => {
  it("constant power", () => {
    const curve = { P0: 3, P20: 3, P80: 3, P100: 3 };
    const eta = integrationETA(curve, 0.6078, 1.6, 2.026, null, null);
    expect(eta).toBeGreaterThan(0);
    expect(eta).toBeLessThan(30);
  });

  it("taper curve", () => {
    const curve = { P0: 3.5, P20: 1.27, P80: 0.79, P100: 0.79 };
    const eta = integrationETA(curve, 0.6078, 1.6, 2.026, null, null);
    expect(eta).toBeGreaterThan(0);
    expect(eta).toBeGreaterThan(25);
  });

  it("zero remaining", () => {
    const curve = { P0: 3, P20: 3, P80: 3, P100: 3 };
    expect(integrationETA(curve, 1.6, 1.6, 2.026, null, null)).toBe(0);
    expect(integrationETA(curve, 1.7, 1.6, 2.026, null, null)).toBe(0);
  });

  it("zero capacity", () => {
    const curve = { P0: 3, P20: 3, P80: 3, P100: 3 };
    expect(integrationETA(curve, 0.5, 1, 0, null, null)).toBe(0);
  });

  it("empty curve", () => {
    const curve = { P0: 0, P20: 0, P80: 0, P100: 0 };
    expect(integrationETA(curve, 0.5, 1, 2.026, null, null)).toBe(0);
  });

  it("matches time0to100min", () => {
    // Must use effectiveCapacityKwh (≈1.583 kWh), not nominal 2.026 kWh.
    // Using nominal capacity causes P0 ≈ 8 W (near-zero soft start) because
    // the average 0-20% power barely exceeds the full charger output.
    const effCap = effectiveCapacityKwh(2.026, 600, 95); // ≈ 1.583 kWh
    const curve = buildStaticCurve({
      capacityKwh: effCap,
      chargerOutputW: 600,
      chargingEfficiency: 0.8,
      time0to80Min: 175,
      time0to100Min: 250,
      time20to80Min: 95,
      time20to100Min: 155,
    });
    const eta = integrationETA(curve, 0, effCap, effCap, null, null);
    expect(eta).toBeGreaterThan(0);
    expect(eta).toBeLessThan(400);
  });

  it("smoothing", () => {
    const curve = { P0: 4, P20: 2, P80: 1, P100: 0.5 };
    const eta = integrationETA(curve, 0.2026, 0.6078, 2.026, null, null);
    expect(eta).toBeGreaterThan(0);
    expect(eta).toBeLessThan(100);
  });

  it("partial static data", () => {
    const curve = { P0: 1, P20: 1.5, P80: 0.8, P100: 0.5 };
    const capacity = 5;
    const cvTime = 90;

    const eta = integrationETA(curve, 0, capacity, capacity, null, cvTime);
    expect(eta).toBeGreaterThan(250);

    const time0to20 = 120;
    const etaBoth = integrationETA(
      curve,
      0,
      capacity,
      capacity,
      time0to20,
      cvTime,
    );
    expect(Math.abs(etaBoth - 360)).toBeLessThan(30);
  });
});

describe("powerAtSOC", () => {
  it("interpolates in 0-20% range", () => {
    const curve = { P0: 3, P20: 2, P80: 1, P100: 0.5 };
    expect(powerAtSOC(curve, 0)).toBe(3);
    expect(powerAtSOC(curve, 10)).toBeCloseTo(2.5);
    expect(powerAtSOC(curve, 20)).toBe(2);
  });

  it("interpolates in 20-80% range", () => {
    const curve = { P0: 3, P20: 2, P80: 1, P100: 0.5 };
    expect(powerAtSOC(curve, 20)).toBe(2);
    expect(powerAtSOC(curve, 50)).toBeCloseTo(1.5);
    expect(powerAtSOC(curve, 80)).toBe(1);
  });

  it("interpolates in 80-100% range", () => {
    const curve = { P0: 3, P20: 2, P80: 1, P100: 0.5 };
    expect(powerAtSOC(curve, 80)).toBe(1);
    expect(powerAtSOC(curve, 90)).toBeCloseTo(0.75);
    expect(powerAtSOC(curve, 100)).toBe(0.5);
  });

  it("returns null for out of bounds", () => {
    const curve = { P0: 3, P20: 2, P80: 1, P100: 0.5 };
    expect(powerAtSOC(curve, -1)).toBeNull();
    expect(powerAtSOC(curve, 101)).toBeNull();
  });

  it("returns null for empty curve", () => {
    const curve = { P0: 0, P20: 0, P80: 0, P100: 0 };
    expect(powerAtSOC(curve, 50)).toBeNull();
  });

  it("handles partial curve P0=0", () => {
    const curve = { P0: 0, P20: 2, P80: 1, P100: 0.5 };
    expect(powerAtSOC(curve, 10)).toBe(2);
  });

  it("handles partial curve P80=0", () => {
    const curve = { P0: 3, P20: 2, P80: 0, P100: 0.5 };
    expect(powerAtSOC(curve, 50)).toBe(2);
  });
});

describe("computeCVTimeMin", () => {
  it.each([
    {
      name: "T20to100 - T20to80 (preferred)",
      t0100: null,
      t080: null,
      t20100: 240,
      t2080: 150,
      want: 90,
    },
    {
      name: "T0to100 - T0to80 fallback when T20 nil",
      t0100: 360,
      t080: 300,
      t20100: null,
      t2080: null,
      want: 60,
    },
    {
      name: "T20to100 preferred over T0to100 when both available",
      t0100: 360,
      t080: 300,
      t20100: 240,
      t2080: 150,
      want: 90,
    },
    {
      name: "all nil",
      t0100: null,
      t080: null,
      t20100: null,
      t2080: null,
      want: null,
    },
    {
      name: "T0to80=0 treated as missing, falls back to T20",
      t0100: 360,
      t080: 0,
      t20100: 240,
      t2080: 150,
      want: 90,
    },
    {
      name: "T0to80=0 with no T20 data → nil",
      t0100: 360,
      t080: 0,
      t20100: null,
      t2080: null,
      want: null,
    },
    {
      name: "T20to80=0 treated as missing, falls back to T0to",
      t0100: 360,
      t080: 300,
      t20100: 240,
      t2080: 0,
      want: 60,
    },
    {
      name: "T20to100 <= T20to80 → falls back to T0to",
      t0100: 360,
      t080: 300,
      t20100: 150,
      t2080: 200,
      want: 60,
    },
  ] as const)("$name", ({ t0100, t080, t20100, t2080, want }) => {
    const got = computeCVTimeMin(t0100, t080, t20100, t2080);
    expect(got).toBe(want);
  });
});

describe("computeTime0to20Min", () => {
  it.each([
    {
      name: "T0to80 - T20to80 (preferred, excludes CV variance)",
      t0100: null,
      t080: 270,
      t20100: null,
      t2080: 150,
      want: 120,
    },
    {
      name: "T0to100 - T20to100 fallback when T0to80 nil",
      t0100: 360,
      t080: null,
      t20100: 240,
      t2080: null,
      want: 120,
    },
    {
      name: "T0to80 preferred over T0to100 when both available",
      t0100: 360,
      t080: 270,
      t20100: 240,
      t2080: 150,
      want: 120,
    },
    {
      name: "all nil",
      t0100: null,
      t080: null,
      t20100: null,
      t2080: null,
      want: null,
    },
    {
      name: "T20to80=0 treated as missing, falls back to T0to100",
      t0100: 360,
      t080: 270,
      t20100: 240,
      t2080: 0,
      want: 120,
    },
    {
      name: "T0to80=0 with no T20to80 → falls back to T0to100",
      t0100: 360,
      t080: 0,
      t20100: 240,
      t2080: null,
      want: 120,
    },
    {
      name: "T0to80=0 with no T0to100 data → nil",
      t0100: null,
      t080: 0,
      t20100: null,
      t2080: 150,
      want: null,
    },
    {
      name: "T0to100 <= T20to100 invalid → falls back to T0to80",
      t0100: 240,
      t080: 270,
      t20100: 360,
      t2080: 150,
      want: 120,
    },
  ] as const)("$name", ({ t0100, t080, t20100, t2080, want }) => {
    const got = computeTime0to20Min(t0100, t080, t20100, t2080);
    expect(got).toBe(want);
  });
});

describe("effectiveCapacityKwh", () => {
  it("returns nominal when no time20to80", () => {
    expect(effectiveCapacityKwh(2.026, 600, null)).toBe(2.026);
  });

  it("returns usable when within validation range", () => {
    const result = effectiveCapacityKwh(2.026, 600, 95);
    expect(result).toBeCloseTo(1.583, 2);
  });

  it("returns nominal when usable is out of range", () => {
    const result = effectiveCapacityKwh(2.026, 10000, 95);
    expect(result).toBe(2.026);
  });
});

describe("hasCurveData", () => {
  it("true when time20to80 set", () => {
    expect(
      hasCurveData({
        capacityKwh: 2.026,
        chargerOutputW: 0,
        chargingEfficiency: 0.8,
        time0to80Min: null,
        time0to100Min: null,
        time20to80Min: 95,
        time20to100Min: null,
      }),
    ).toBe(true);
  });

  it("true when chargerOutputW set", () => {
    expect(
      hasCurveData({
        capacityKwh: 2.026,
        chargerOutputW: 600,
        chargingEfficiency: 0.8,
        time0to80Min: null,
        time0to100Min: null,
        time20to80Min: null,
        time20to100Min: null,
      }),
    ).toBe(true);
  });

  it("true when capacityKwh set", () => {
    expect(
      hasCurveData({
        capacityKwh: 2.026,
        chargerOutputW: 0,
        chargingEfficiency: 0.8,
        time0to80Min: null,
        time0to100Min: null,
        time20to80Min: null,
        time20to100Min: null,
      }),
    ).toBe(true);
  });

  it("false when nothing set", () => {
    expect(
      hasCurveData({
        capacityKwh: 0,
        chargerOutputW: 0,
        chargingEfficiency: 0.8,
        time0to80Min: null,
        time0to100Min: null,
        time20to80Min: null,
        time20to100Min: null,
      }),
    ).toBe(false);
  });
});

describe("calculateETA", () => {
  const baseVehicle = {
    capacityKwh: 2.026,
    chargerOutputW: 600,
    chargingEfficiency: 0.8,
    time0to80Min: null as number | null,
    time0to100Min: null as number | null,
    time20to80Min: null as number | null,
    time20to100Min: null as number | null,
  };

  it("static curve", () => {
    const v = { ...baseVehicle, time20to80Min: 95 };
    const eta = calculateETA({ ...v, currentPercent: 30, targetPercent: 80 });
    expect(eta).not.toBeNull();
    if (eta !== null) expect(eta).toBeGreaterThan(0);
  });

  it("no curve data", () => {
    const eta = calculateETA({
      ...baseVehicle,
      capacityKwh: 0,
      currentPercent: 30,
      targetPercent: 80,
    });
    expect(eta).toBeNull();
  });

  it("current equals target", () => {
    const v = { ...baseVehicle, time20to80Min: 95 };
    const eta = calculateETA({ ...v, currentPercent: 50, targetPercent: 50 });
    expect(eta).toBeNull();
  });

  it("current exceeds target", () => {
    const v = { ...baseVehicle, time20to80Min: 95 };
    const eta = calculateETA({ ...v, currentPercent: 70, targetPercent: 50 });
    expect(eta).toBeNull();
  });

  it("RM1 realistic", () => {
    const v = {
      ...baseVehicle,
      time0to80Min: 175,
      time0to100Min: 250,
      time20to80Min: 95,
      time20to100Min: 155,
    };
    const eta = calculateETA({ ...v, currentPercent: 30, targetPercent: 80 });
    expect(eta).not.toBeNull();
    if (eta !== null) {
      expect(eta).toBeGreaterThan(50);
      expect(eta).toBeLessThan(150);
    }
  });

  it("RM2 from DB", () => {
    const v = {
      capacityKwh: 5.46,
      chargerOutputW: 1200,
      chargingEfficiency: 0.8,
      time0to80Min: 0,
      time0to100Min: 360,
      time20to80Min: 150,
      time20to100Min: 240,
    };

    const eta = calculateETA({ ...v, currentPercent: 20, targetPercent: 80 });
    expect(eta).not.toBeNull();
    if (eta !== null) expect(eta).toBeCloseTo(150, 0);

    const eta100 = calculateETA({
      ...v,
      currentPercent: 20,
      targetPercent: 100,
    });
    expect(eta100).not.toBeNull();
    if (eta100 !== null) expect(eta100).toBeCloseTo(240, 0);

    const etaFull = calculateETA({
      ...v,
      currentPercent: 0,
      targetPercent: 100,
    });
    expect(etaFull).not.toBeNull();
    if (etaFull !== null) expect(etaFull).toBeCloseTo(360, 0);

    const etaShort = calculateETA({
      ...v,
      currentPercent: 5,
      targetPercent: 15,
    });
    expect(etaShort).not.toBeNull();
    if (etaShort !== null) expect(etaShort).toBeCloseTo(60, 0);
  });
});

// ─── helpers ──────────────────────────────────────────────────────────────────

function invariants(curve: ReturnType<typeof buildStaticCurve>, label: string) {
  expect(curve.P0, `${label}: P0 ≥ 0`).toBeGreaterThanOrEqual(0);
  expect(curve.P20, `${label}: P20 ≥ 0`).toBeGreaterThanOrEqual(0);
  expect(curve.P80, `${label}: P80 ≥ 0`).toBeGreaterThanOrEqual(0);
  expect(curve.P100, `${label}: P100 ≥ 0`).toBeGreaterThanOrEqual(0);
  expect(curve.P0, `${label}: P0 ≤ P20`).toBeLessThanOrEqual(curve.P20 + 1e-9);
  expect(curve.P20, `${label}: P20 = P80`).toBeCloseTo(curve.P80, 9);
  expect(curve.P100, `${label}: P100 ≤ P80`).toBeLessThanOrEqual(
    curve.P80 + 1e-9,
  );
}

// ─── buildStaticCurve ─────────────────────────────────────────────────────────

describe("buildStaticCurve", () => {
  it("RM1 full data - correct physics with effective capacity", () => {
    const effCap = effectiveCapacityKwh(2.026, 600, 95); // ≈ 1.583 kWh
    const c = buildStaticCurve({
      capacityKwh: effCap,
      chargerOutputW: 600,
      chargingEfficiency: 0.8,
      time0to80Min: 175,
      time0to100Min: 250,
      time20to80Min: 95,
      time20to100Min: 155,
    });
    // P20 = P80 = charger ceiling (600 W = 0.6 kW)
    expect(c.P20).toBeCloseTo(0.6, 3);
    expect(c.P80).toBeCloseTo(0.6, 3);
    // P100 ≈ 33 W: 2 × (effCap×0.2 / (60min/60)) − 0.6
    expect(c.P100).toBeGreaterThan(0.02);
    expect(c.P100).toBeLessThan(0.05);
    // P0 > 0 and below charger ceiling
    expect(c.P0).toBeGreaterThan(0);
    expect(c.P0).toBeLessThanOrEqual(0.6);
    invariants(c, "RM1 full");
  });

  it("RM2 - no t0to80, falls back to t0to100−t20to100 for soft-start", () => {
    const effCap = effectiveCapacityKwh(5.46, 1200, 150); // = 5.0 kWh
    const c = buildStaticCurve({
      capacityKwh: effCap,
      chargerOutputW: 1200,
      chargingEfficiency: 0.8,
      time0to80Min: 0,
      time0to100Min: 360,
      time20to80Min: 150,
      time20to100Min: 240,
    });
    expect(c.P20).toBeCloseTo(1.2, 3);
    expect(c.P80).toBeCloseTo(1.2, 3);
    expect(c.P100).toBeGreaterThan(0);
    expect(c.P0).toBeGreaterThan(0);
    invariants(c, "RM2");
  });

  it("only t0to100 and t0to80 - derives CC and CV from 0-based times", () => {
    // t0to80=175, t0to100=250, charger=600W, capacity=2.026
    // pCC = min(deriveP20(no t2080 → 0, fallback pMax=0.6), 0.6) = 0.6
    // cvTime = t0to100 - t0to80 = 75 min
    // p100 = 2×(2.026×0.2/(75/60)) − 0.6 = 2×0.324 − 0.6 = 0.048
    const c = buildStaticCurve({
      capacityKwh: 2.026,
      chargerOutputW: 600,
      chargingEfficiency: 0.8,
      time0to80Min: 175,
      time0to100Min: 250,
      time20to80Min: null,
      time20to100Min: null,
    });
    expect(c.P20).toBeCloseTo(0.6, 3);
    expect(c.P80).toBeCloseTo(0.6, 3);
    expect(c.P100).toBeGreaterThan(0); // from t0-based cv time
    expect(c.P0).toBeGreaterThan(0);
    invariants(c, "t0-only");
  });

  it("pack specs only (no time data) - P100 from physics", () => {
    // P100 = 58.8V × 600mA / 1_000_000 = 0.03528 kW ≈ 35 W
    const c = buildStaticCurve({
      capacityKwh: 2.026,
      chargerOutputW: 600,
      chargingEfficiency: 0.8,
      time0to80Min: null,
      time0to100Min: null,
      time20to80Min: null,
      time20to100Min: null,
      packVoltageMaxV: 58.8,
      packCutoffCurrentMa: 600,
    });
    expect(c.P20).toBeCloseTo(0.6, 3);
    expect(c.P80).toBeCloseTo(0.6, 3);
    expect(c.P100).toBeCloseTo((58.8 * 600) / 1_000_000, 6);
    // P0 falls back to pCC × 0.5 = 0.3 kW
    expect(c.P0).toBeCloseTo(0.3, 3);
    invariants(c, "pack specs only");
  });

  it("pack specs ignored when time data provides P100", () => {
    const effCap = effectiveCapacityKwh(2.026, 600, 95);
    const withPack = buildStaticCurve({
      capacityKwh: effCap,
      chargerOutputW: 600,
      chargingEfficiency: 0.8,
      time0to80Min: 175,
      time0to100Min: 250,
      time20to80Min: 95,
      time20to100Min: 155,
      packVoltageMaxV: 58.8,
      packCutoffCurrentMa: 600,
    });
    const withoutPack = buildStaticCurve({
      capacityKwh: effCap,
      chargerOutputW: 600,
      chargingEfficiency: 0.8,
      time0to80Min: 175,
      time0to100Min: 250,
      time20to80Min: 95,
      time20to100Min: 155,
    });
    // time-based P100 wins; pack spec value (35W) should not override
    expect(withPack.P100).toBeCloseTo(withoutPack.P100, 6);
    invariants(withPack, "pack ignored");
  });

  it("CC time only - P100 = 0, P0 from default ratio", () => {
    const c = buildStaticCurve({
      capacityKwh: 3,
      chargerOutputW: 800,
      chargingEfficiency: 0.8,
      time0to80Min: null,
      time0to100Min: null,
      time20to80Min: 135, // derives pCC = 3×0.6/(135/60) = 0.8 kW = charger max
      time20to100Min: null,
    });
    expect(c.P20).toBeCloseTo(0.8, 3);
    expect(c.P80).toBeCloseTo(0.8, 3);
    expect(c.P100).toBe(0);
    // P0 = pCC × defaultSoftStartRatio = 0.8 × 0.5 = 0.4
    expect(c.P0).toBeCloseTo(0.4, 3);
    invariants(c, "CC time only");
  });

  it("charger+capacity only - P100 = 0, P0 from default ratio", () => {
    const c = buildStaticCurve({
      capacityKwh: 3,
      chargerOutputW: 800,
      chargingEfficiency: 0.8,
      time0to80Min: null,
      time0to100Min: null,
      time20to80Min: null,
      time20to100Min: null,
    });
    expect(c.P20).toBeCloseTo(0.8, 3);
    expect(c.P80).toBeCloseTo(0.8, 3);
    expect(c.P100).toBe(0);
    expect(c.P0).toBeCloseTo(0.4, 3); // 0.8 × 0.5
    invariants(c, "charger+cap only");
  });

  it("CC power exceeding charger limit is capped", () => {
    // t20to80 implying P20 > charger: 3kWh × 0.6 / (0.5h) = 3.6 kW >> 800W
    const c = buildStaticCurve({
      capacityKwh: 3,
      chargerOutputW: 800,
      chargingEfficiency: 0.8,
      time0to80Min: null,
      time0to100Min: null,
      time20to80Min: 30, // derives pCC = 3×0.6/0.5 = 3.6 kW → capped to 0.8
      time20to100Min: null,
    });
    expect(c.P20).toBeCloseTo(0.8, 3);
    expect(c.P80).toBeCloseTo(0.8, 3);
    invariants(c, "capped CC");
  });

  it("pre-CC time implying negative P0 falls back gracefully", () => {
    // t0to80=100 < t20to80=95 → t0to20=5min (very short) → P0 formula gives high then negative
    // With t0to20=5min: energy/hours=0.3166/0.0833=3.8 kW >> charger, then 2×3.8-0.6=7kW
    // After capping: P0=0.6=P20 (enforceMonotonicity clamps to P20 if above)
    const effCap = effectiveCapacityKwh(2.026, 600, 95);
    const c = buildStaticCurve({
      capacityKwh: effCap,
      chargerOutputW: 600,
      chargingEfficiency: 0.8,
      time0to80Min: 100, // only 5 min before t20to80=95
      time0to100Min: null,
      time20to80Min: 95,
      time20to100Min: null,
    });
    expect(c.P0).toBeGreaterThanOrEqual(0);
    expect(c.P0).toBeLessThanOrEqual(c.P20 + 1e-9);
    invariants(c, "short pre-CC");
  });
});

// ─── dynamicEffectiveCapacityKwh ─────────────────────────────────────────────

describe("dynamicEffectiveCapacityKwh", () => {
  const specCap = 2.026;
  const charger = 600;
  const t2080 = 95;
  const specEff = effectiveCapacityKwh(specCap, charger, t2080); // ≈ 1.583

  it("uses observed data when in CC range with sufficient delta", () => {
    // 0.5 kWh over 30% SOC → observed cap = 0.5/0.30 = 1.667 kWh
    const result = dynamicEffectiveCapacityKwh(
      specCap,
      charger,
      t2080,
      0.5,
      20,
      50,
    );
    expect(result).toBeCloseTo(0.5 / 0.3, 3);
  });

  it("falls back when energyAddedKwh below minimum threshold", () => {
    const result = dynamicEffectiveCapacityKwh(
      specCap,
      charger,
      t2080,
      0.05,
      20,
      50, // only 50 Wh - below 100 Wh minimum
    );
    expect(result).toBeCloseTo(specEff, 3);
  });

  it("falls back when SOC delta below minimum threshold", () => {
    const result = dynamicEffectiveCapacityKwh(
      specCap,
      charger,
      t2080,
      0.5,
      20,
      25, // only 5% delta - below 10% minimum
    );
    expect(result).toBeCloseTo(specEff, 3);
  });

  it("falls back when start SOC is below 20% (mixed pre-CC/CC)", () => {
    const result = dynamicEffectiveCapacityKwh(
      specCap,
      charger,
      t2080,
      0.5,
      10,
      40, // starts at 10% (pre-CC region)
    );
    expect(result).toBeCloseTo(specEff, 3);
  });

  it("falls back when current SOC exceeds 80% (mixed CC/CV)", () => {
    const result = dynamicEffectiveCapacityKwh(
      specCap,
      charger,
      t2080,
      0.5,
      60,
      90, // ends at 90% (CV region)
    );
    expect(result).toBeCloseTo(specEff, 3);
  });

  it("falls back when observed capacity is implausibly large", () => {
    // 10 kWh over 10% SOC → 100 kWh - way above 1.5× spec
    const result = dynamicEffectiveCapacityKwh(
      specCap,
      charger,
      t2080,
      10,
      20,
      30,
    );
    expect(result).toBeCloseTo(specEff, 3);
  });

  it("falls back when observed capacity is implausibly small", () => {
    // 0.01 kWh over 20% → 0.05 kWh - way below 0.5× spec
    const result = dynamicEffectiveCapacityKwh(
      specCap,
      charger,
      t2080,
      0.12,
      20,
      60, // 0.12/0.40 = 0.3 kWh << 0.5 × 1.583
    );
    expect(result).toBeCloseTo(specEff, 3);
  });

  it("falls back when energyAddedKwh is null", () => {
    const result = dynamicEffectiveCapacityKwh(
      specCap,
      charger,
      t2080,
      null,
      20,
      50,
    );
    expect(result).toBeCloseTo(specEff, 3);
  });

  it("falls back when startPercent is null", () => {
    const result = dynamicEffectiveCapacityKwh(
      specCap,
      charger,
      t2080,
      0.5,
      null,
      50,
    );
    expect(result).toBeCloseTo(specEff, 3);
  });
});
