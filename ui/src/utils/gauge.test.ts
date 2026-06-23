import type { TariffSettings } from "@/lib/schemas";

import { describe, it, expect } from "vitest";

import {
  percentageToAngle,
  angleToPercentage,
  getCoordinatesForPercentage,
  distance,
  activeRatePence,
  formatCost,
  formatDuration,
  formatEstimatedCost,
  formatEstimatedTime,
  formatPenceCost,
  formatPower,
  formatRange,
  gaugeAngleFromOffset,
  isAngleInGap,
  gaugeAngleToPercentage,
  angularDistance,
  DefaultCostPerKwh,
} from "./gauge";

const START_ANGLE_RAD = (135 * Math.PI) / 180;
const TOTAL_ARC_RAD = (270 * Math.PI) / 180;

describe("percentageToAngle", () => {
  it("converts 0% to start angle (135deg)", () => {
    const result = percentageToAngle(0);
    expect(result).toBeCloseTo((135 * Math.PI) / 180, 5);
  });

  it("converts 50% to middle angle (270deg)", () => {
    const result = percentageToAngle(50);
    expect(result).toBeCloseTo((270 * Math.PI) / 180, 5);
  });

  it("converts 100% to end angle (405deg)", () => {
    const result = percentageToAngle(100);
    expect(result).toBeCloseTo((405 * Math.PI) / 180, 5);
  });

  it("converts 25% correctly", () => {
    const result = percentageToAngle(25);
    expect(result).toBeCloseTo(((135 + (25 / 100) * 270) * Math.PI) / 180, 5);
  });

  it("converts 75% correctly", () => {
    const result = percentageToAngle(75);
    expect(result).toBeCloseTo(((135 + (75 / 100) * 270) * Math.PI) / 180, 5);
  });
});

describe("angleToPercentage", () => {
  it("converts 135deg back to 0%", () => {
    const angle = (135 * Math.PI) / 180;
    const result = angleToPercentage(angle);
    expect(result).toBeCloseTo(0, 5);
  });

  it("converts 270deg back to 50%", () => {
    const angle = (270 * Math.PI) / 180;
    const result = angleToPercentage(angle);
    expect(result).toBeCloseTo(50, 5);
  });

  it("converts 405deg back to 100%", () => {
    const angle = (405 * Math.PI) / 180;
    const result = angleToPercentage(angle);
    expect(result).toBeCloseTo(100, 5);
  });
});

describe("getCoordinatesForPercentage", () => {
  it("returns correct coordinates for 0%", () => {
    const result = getCoordinatesForPercentage(0, 320, 320, 134.4, 114.24);
    expect(result.x).toBeGreaterThan(0);
    expect(result.y).toBeGreaterThan(0);
  });

  it("returns correct coordinates for 50%", () => {
    const result = getCoordinatesForPercentage(50, 320, 320, 134.4, 114.24);
    expect(result.x).toBeGreaterThan(0);
    expect(result.y).toBeGreaterThan(0);
  });

  it("returns correct coordinates for 100%", () => {
    const result = getCoordinatesForPercentage(100, 320, 320, 134.4, 114.24);
    expect(result.x).toBeGreaterThan(0);
    expect(result.y).toBeGreaterThan(0);
  });
});

describe("distance", () => {
  it("calculates distance between two points", () => {
    const result = distance(0, 0, 3, 4);
    expect(result).toBeCloseTo(5, 5);
  });

  it("calculates distance when points are same", () => {
    const result = distance(5, 5, 5, 5);
    expect(result).toBeCloseTo(0, 5);
  });

  it("calculates horizontal distance", () => {
    const result = distance(0, 0, 10, 0);
    expect(result).toBeCloseTo(10, 5);
  });

  it("calculates vertical distance", () => {
    const result = distance(0, 0, 0, 10);
    expect(result).toBeCloseTo(10, 5);
  });
});

describe("formatPower", () => {
  it("formats watts when below 1000", () => {
    expect(formatPower(500)).toBe("500 W");
  });

  it("formats watts with rounding", () => {
    expect(formatPower(543)).toBe("543 W");
  });

  it("formats as kW when 1000 or above", () => {
    expect(formatPower(1000)).toBe("1.00 kW");
  });

  it("formats kW with two decimal places", () => {
    expect(formatPower(39312)).toBe("39.31 kW");
  });

  it("formats exactly 1.5 kW", () => {
    expect(formatPower(1500)).toBe("1.50 kW");
  });

  it("formats 0 W", () => {
    expect(formatPower(0)).toBe("0 W");
  });

  it("formats 999 W (below threshold)", () => {
    expect(formatPower(999)).toBe("999 W");
  });

  it("formats 999.5 W as 1000 W (Math.round rounds up)", () => {
    expect(formatPower(999.5)).toBe("1000 W");
  });

  it("formats 1000.5 W as 1.00 kW (above threshold)", () => {
    expect(formatPower(1000.5)).toBe("1.00 kW");
  });
});

describe("percentageToAngle edge cases", () => {
  it("clamps negative percentage to 0", () => {
    const result = percentageToAngle(-10);
    expect(result).toBeCloseTo((135 * Math.PI) / 180, 5);
  });

  it("clamps percentage above 100 to 100", () => {
    const result = percentageToAngle(150);
    expect(result).toBeCloseTo(((135 + 270) * Math.PI) / 180, 5);
  });
});

describe("angleToPercentage edge cases", () => {
  it("handles negative angle (no clamping)", () => {
    const result = angleToPercentage(-Math.PI / 4);
    // -45deg → normalized to 315deg → 315-135=180 → 180/270*100 = 66.67%
    expect(result).toBeCloseTo(66.67, 1);
  });

  it("handles 0 radians (no clamping)", () => {
    const result = angleToPercentage(0);
    // 0deg → normalized to 0deg → 0-135=-135 → 225 → 225/270*100 = 83.33%
    expect(result).toBeCloseTo(83.33, 1);
  });
});

describe("distance edge cases", () => {
  it("handles negative coordinates", () => {
    const result = distance(-5, -5, 5, 5);
    expect(result).toBeCloseTo(Math.sqrt(200), 5);
  });
});

describe("isAngleInGap", () => {
  it("returns false for angles within the arc (135°-405°)", () => {
    expect(isAngleInGap(START_ANGLE_RAD, START_ANGLE_RAD, TOTAL_ARC_RAD)).toBe(
      false,
    );
    expect(isAngleInGap(Math.PI / 6, START_ANGLE_RAD, TOTAL_ARC_RAD)).toBe(
      false,
    );
    expect(isAngleInGap(Math.PI, START_ANGLE_RAD, TOTAL_ARC_RAD)).toBe(false);
    expect(
      isAngleInGap((3 * Math.PI) / 2, START_ANGLE_RAD, TOTAL_ARC_RAD),
    ).toBe(false);
  });

  it("returns true for angles in the gap region (45°-135°)", () => {
    expect(isAngleInGap(Math.PI / 3, START_ANGLE_RAD, TOTAL_ARC_RAD)).toBe(
      true,
    );
    expect(isAngleInGap(Math.PI / 2, START_ANGLE_RAD, TOTAL_ARC_RAD)).toBe(
      true,
    );
  });

  it("returns false for the start angle exactly", () => {
    expect(isAngleInGap(START_ANGLE_RAD, START_ANGLE_RAD, TOTAL_ARC_RAD)).toBe(
      false,
    );
  });

  it("returns false for the end angle exactly", () => {
    expect(
      isAngleInGap(
        START_ANGLE_RAD + TOTAL_ARC_RAD,
        START_ANGLE_RAD,
        TOTAL_ARC_RAD,
      ),
    ).toBe(false);
  });

  it("handles negative angles (-45° normalizes to arc)", () => {
    expect(isAngleInGap(-Math.PI / 4, START_ANGLE_RAD, TOTAL_ARC_RAD)).toBe(
      false,
    );
  });

  it("handles angles beyond 2π", () => {
    expect(
      isAngleInGap(2 * Math.PI + Math.PI / 3, START_ANGLE_RAD, TOTAL_ARC_RAD),
    ).toBe(true);
  });
});

describe("gaugeAngleToPercentage", () => {
  it("converts start angle to 0%", () => {
    expect(
      gaugeAngleToPercentage(START_ANGLE_RAD, START_ANGLE_RAD, TOTAL_ARC_RAD),
    ).toBeCloseTo(0, 5);
  });

  it("converts middle angle (270°) to 50%", () => {
    expect(
      gaugeAngleToPercentage(
        (270 * Math.PI) / 180,
        START_ANGLE_RAD,
        TOTAL_ARC_RAD,
      ),
    ).toBeCloseTo(50, 5);
  });

  it("converts end angle to 100%", () => {
    expect(
      gaugeAngleToPercentage(
        START_ANGLE_RAD + TOTAL_ARC_RAD,
        START_ANGLE_RAD,
        TOTAL_ARC_RAD,
      ),
    ).toBeCloseTo(100, 5);
  });

  it("snaps gap angle (π/3) to 100%", () => {
    expect(
      gaugeAngleToPercentage(Math.PI / 3, START_ANGLE_RAD, TOTAL_ARC_RAD),
    ).toBeCloseTo(100, 5);
  });

  it("maps angle 0 (315° normalized, on arc) to ~83.33%", () => {
    expect(
      gaugeAngleToPercentage(0, START_ANGLE_RAD, TOTAL_ARC_RAD),
    ).toBeCloseTo(83.33, 1);
  });

  it("handles 25% correctly", () => {
    const angle =
      (START_ANGLE_RAD + (25 / 100) * TOTAL_ARC_RAD) % (2 * Math.PI);
    expect(
      gaugeAngleToPercentage(angle, START_ANGLE_RAD, TOTAL_ARC_RAD),
    ).toBeCloseTo(25, 5);
  });

  it("handles 75% correctly", () => {
    const angle =
      (START_ANGLE_RAD + (75 / 100) * TOTAL_ARC_RAD) % (2 * Math.PI);
    expect(
      gaugeAngleToPercentage(angle, START_ANGLE_RAD, TOTAL_ARC_RAD),
    ).toBeCloseTo(75, 5);
  });
});

describe("angularDistance", () => {
  it("returns 0 for identical angles", () => {
    expect(angularDistance(Math.PI / 2, Math.PI / 2)).toBeCloseTo(0, 5);
  });

  it("returns shortest distance across 0 boundary", () => {
    expect(angularDistance(0.1, 2 * Math.PI - 0.1)).toBeCloseTo(0.2, 5);
  });

  it("returns π for opposite angles", () => {
    expect(angularDistance(0, Math.PI)).toBeCloseTo(Math.PI, 5);
  });

  it("handles negative angles", () => {
    expect(angularDistance(-Math.PI / 4, Math.PI / 4)).toBeCloseTo(
      Math.PI / 2,
      5,
    );
  });

  it("handles angles beyond 2π", () => {
    expect(angularDistance(2 * Math.PI + 0.5, 0.5)).toBeCloseTo(0, 5);
  });
});

describe("gaugeAngleFromOffset", () => {
  it("returns correct angle at center", () => {
    const angle = gaugeAngleFromOffset(150, 150, 300, 300);
    // At center, atan2(0, 0) = 0
    expect(angle).toBeCloseTo(0, 5);
  });

  it("returns correct angle at bottom of gauge (50%)", () => {
    // Bottom of a 300px gauge: offset (150, 300) → (0, 150) from center → 90°
    const angle = gaugeAngleFromOffset(150, 300, 300, 300);
    expect(angle).toBeCloseTo(Math.PI / 2, 5);
  });

  it("returns correct angle at top of gauge (in gap region)", () => {
    // Top of a 300px gauge: offset (150, 0) → (0, -150) from center → 270° (3π/2)
    const angle = gaugeAngleFromOffset(150, 0, 300, 300);
    expect(angle).toBeCloseTo((3 * Math.PI) / 2, 5);
  });

  it("returns correct angle at left edge (0%)", () => {
    // Left of a 300px gauge: offset (0, 150) → (-150, 0) from center
    // atan2(0, -150) = π → 180°. But gauge starts at 135°, so 180° is near 0%
    const angle = gaugeAngleFromOffset(0, 150, 300, 300);
    expect(angle).toBeCloseTo(Math.PI, 5);
  });

  it("is scale-invariant: same angle at 300px and 420px", () => {
    // This is the regression test for the global drag handler bug.
    // The old code computed atan2(y*scale - viewBoxCenter, x*scale - viewBoxCenter)
    // which produced wrong angles when the element size differed from the viewBox.
    const angle300 = gaugeAngleFromOffset(150, 300, 300, 300);
    const angle420 = gaugeAngleFromOffset(210, 420, 420, 420);
    expect(angle420).toBeCloseTo(angle300, 5);
  });

  it("is scale-invariant: same angle at 640px and 300px", () => {
    const angle300 = gaugeAngleFromOffset(150, 300, 300, 300);
    const angle640 = gaugeAngleFromOffset(320, 640, 640, 640);
    expect(angle640).toBeCloseTo(angle300, 5);
  });

  it("produces correct angle at different positions across scales", () => {
    // Test multiple positions to ensure scale invariance
    const positions: [number, number][] = [
      [75, 75],
      [225, 75],
      [225, 225],
      [75, 225],
      [150, 0],
      [300, 150],
    ];
    const scales: number[] = [300, 420, 520, 640];

    for (const [bx, by] of positions) {
      const angleBase = gaugeAngleFromOffset(bx, by, 300, 300);
      for (const scale of scales) {
        const sx = (bx / 300) * scale;
        const sy = (by / 300) * scale;
        const angleScaled = gaugeAngleFromOffset(sx, sy, scale, scale);
        expect(angleScaled).toBeCloseTo(angleBase, 4);
      }
    }
  });
});

describe("formatDuration", () => {
  it("formats zero milliseconds", () => {
    expect(formatDuration(0)).toBe("00:00:00");
  });

  it("formats seconds only", () => {
    expect(formatDuration(45000)).toBe("00:00:45");
  });

  it("formats minutes and seconds", () => {
    expect(formatDuration(90000)).toBe("00:01:30");
  });

  it("formats hours, minutes, and seconds", () => {
    expect(formatDuration(3690000)).toBe("01:01:30");
  });

  it("formats large durations", () => {
    expect(formatDuration(7200000)).toBe("02:00:00");
  });

  it("handles sub-second precision by flooring", () => {
    expect(formatDuration(123456)).toBe("00:02:03");
  });

  it("formats exactly one hour", () => {
    expect(formatDuration(3600000)).toBe("01:00:00");
  });

  it("formats exactly one minute", () => {
    expect(formatDuration(60000)).toBe("00:01:00");
  });
});

describe("formatRange", () => {
  it("formats range at 0%", () => {
    expect(formatRange(100, 150, 0)).toBe("0 mi");
  });

  it("formats range at 100%", () => {
    expect(formatRange(100, 150, 100)).toBe("100-150 mi");
  });

  it("formats range at 50%", () => {
    expect(formatRange(100, 150, 50)).toBe("50-75 mi");
  });

  it("formats range at 25%", () => {
    expect(formatRange(100, 150, 25)).toBe("25-38 mi");
  });

  it("formats range at 80%", () => {
    expect(formatRange(100, 150, 80)).toBe("80-120 mi");
  });

  it("returns single value when min equals max", () => {
    expect(formatRange(100, 100, 50)).toBe("50 mi");
  });

  it("handles zero range", () => {
    expect(formatRange(0, 0, 50)).toBe("0 mi");
  });

  it("handles fractional rounding", () => {
    expect(formatRange(100, 150, 33)).toBe("33-50 mi");
  });

  it("handles large range values", () => {
    expect(formatRange(250, 320, 60)).toBe("150-192 mi");
  });
});

describe("formatEstimatedTime", () => {
  it("formats zero minutes", () => {
    expect(formatEstimatedTime(0)).toBe("00:00:00");
  });

  it("formats minutes only", () => {
    expect(formatEstimatedTime(11)).toBe("00:11:00");
  });

  it("formats hours and minutes", () => {
    expect(formatEstimatedTime(131)).toBe("02:11:00");
  });

  it("formats exactly one hour", () => {
    expect(formatEstimatedTime(60)).toBe("01:00:00");
  });

  it("formats large durations", () => {
    expect(formatEstimatedTime(150)).toBe("02:30:00");
  });

  it("handles fractional minutes by flooring", () => {
    expect(formatEstimatedTime(11.5)).toBe("00:11:30");
  });
});

describe("formatCost", () => {
  it("formats cost as pounds", () => {
    expect(formatCost(1, 0.8, 26.11)).toBe("£0.33");
  });

  it("formats cost at exactly 100p as pounds", () => {
    // 3.13 kWh / 0.8 * 25.56p = 100.0p → £1.00
    expect(formatCost(3.13, 0.8, 25.56)).toBe("£1.00");
  });

  it("formats cost above 100p as pounds", () => {
    // 5 kWh battery-side, 0.8 efficiency → 6.25 kWh wall × 30p/kWh = 188p → £1.88
    expect(formatCost(5, 0.8, 30)).toBe("£1.88");
  });

  it("formats cost with zero energy", () => {
    expect(formatCost(0, 0.8, 26.11)).toBe("£0.00");
  });

  it("formats cost with null energy", () => {
    expect(formatCost(null, 0.8, 26.11)).toBe("£0.00");
  });

  it("formats cost with zero rate", () => {
    expect(formatCost(1, 0.8, 0)).toBe("£0.00");
  });

  it("formats cost with small energy", () => {
    expect(formatCost(0.1, 0.8, 26.11)).toBe("£0.03");
  });
});

describe("formatPenceCost", () => {
  it("formats whole pence as pounds", () => {
    expect(formatPenceCost(14)).toBe("£0.14");
    expect(formatPenceCost(1400)).toBe("£14.00");
  });

  it("rounds fractional pence", () => {
    expect(formatPenceCost(46.6)).toBe("£0.47");
  });

  it("returns £0.00 for null, undefined, or negative", () => {
    expect(formatPenceCost(null)).toBe("£0.00");
    expect(formatPenceCost(undefined)).toBe("£0.00");
    expect(formatPenceCost(-5)).toBe("£0.00");
  });
});

describe("activeRatePence", () => {
  const tariff: TariffSettings = {
    baseRatePence: 24,
    offPeakWindows: [{ start: "00:30", end: "04:30", ratePence: 7 }],
  };

  const at = (h: number, m = 0) => new Date(2026, 5, 21, h, m, 0);

  it("returns the off-peak rate inside the window", () => {
    expect(activeRatePence(tariff, at(2))).toBe(7);
  });

  it("returns the base rate outside the window", () => {
    expect(activeRatePence(tariff, at(12))).toBe(24);
  });

  it("treats the window end as exclusive", () => {
    expect(activeRatePence(tariff, at(4, 30))).toBe(24);
  });

  it("handles windows that wrap past midnight", () => {
    const wrap: TariffSettings = {
      baseRatePence: 24,
      offPeakWindows: [{ start: "23:30", end: "05:30", ratePence: 6 }],
    };
    expect(activeRatePence(wrap, at(23, 45))).toBe(6);
    expect(activeRatePence(wrap, at(1))).toBe(6);
    expect(activeRatePence(wrap, at(12))).toBe(24);
  });

  it("falls back to the default rate when no tariff is provided", () => {
    expect(activeRatePence(null, at(2))).toBe(DefaultCostPerKwh);
  });

  it("uses the first matching window", () => {
    const overlapping: TariffSettings = {
      baseRatePence: 24,
      offPeakWindows: [
        { start: "00:30", end: "04:30", ratePence: 7 },
        { start: "01:00", end: "03:00", ratePence: 5 },
      ],
    };
    expect(activeRatePence(overlapping, at(2))).toBe(7);
  });
});

describe("formatEstimatedCost", () => {
  it("formats estimated cost as pounds", () => {
    expect(formatEstimatedCost(28, 80, 5.46, 0.8, 26.11)).toBe("£0.93");
  });

  it("formats estimated cost above 100p as pounds", () => {
    // 0% → 100%, 5.46 kWh → 6.825 kWh wall × 26.11p = 178p → £1.78
    expect(formatEstimatedCost(0, 100, 5.46, 0.8, 26.11)).toBe("£1.78");
  });

  it("returns £0.00 when target equals current", () => {
    expect(formatEstimatedCost(80, 80, 5.46, 0.8, 26.11)).toBe("£0.00");
  });

  it("returns £0.00 when target is below current", () => {
    expect(formatEstimatedCost(80, 50, 5.46, 0.8, 26.11)).toBe("£0.00");
  });

  it("returns £0.00 with zero capacity", () => {
    expect(formatEstimatedCost(20, 80, 0, 0.8, 26.11)).toBe("£0.00");
  });

  it("returns £0.00 with zero rate", () => {
    expect(formatEstimatedCost(20, 80, 5.46, 0.8, 0)).toBe("£0.00");
  });

  it("formats with perfect efficiency", () => {
    expect(formatEstimatedCost(50, 80, 5.46, 1, 25)).toBe("£0.41");
  });
});
