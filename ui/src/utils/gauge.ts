import type { TariffSettings } from "@/lib/schemas";

import {
  GAUGE_ARC_SPAN_DEG,
  GAUGE_ARC_START_ANGLE_DEG,
  GAUGE_FULL_ROTATION_DEG,
  PERCENT_SCALE,
  WATT_TO_KILO_WATT_THRESHOLD,
} from "@/lib/constants";

/**
 * Converts a percentage to polar coordinates on a circle.
 * Given a center point (cx, cy) and radius, returns the x,y position
 * at that percentage around the gauge arc.
 *
 * @param cx - Center x coordinate
 * @param cy - Center y coordinate
 * @param r - Radius
 * @param pct - Percentage (0-100)
 * @returns Object with x and y coordinates
 */
export const polar = (
  cx: number,
  cy: number,
  r: number,
  pct: number,
): { x: number; y: number } => {
  const angle = percentageToAngle(pct);
  return {
    x: +(cx + Math.cos(angle) * r).toFixed(4),
    y: +(cy + Math.sin(angle) * r).toFixed(4),
  };
};

/**
 * Converts a percentage value (0-100) to an angle in radians.
 * The speedometer gauge spans 270 degrees, starting at 135 degrees.
 *
 * @param percentage - The percentage value (0-100)
 * @returns The angle in radians
 */
export const percentageToAngle = (percentage: number): number => {
  const clamped = Math.max(0, Math.min(PERCENT_SCALE, percentage));
  return (
    (GAUGE_ARC_START_ANGLE_DEG +
      (clamped / PERCENT_SCALE) * GAUGE_ARC_SPAN_DEG) *
    (Math.PI / 180)
  );
};

/**
 * Converts an angle in radians to a percentage value (0-100).
 *
 * @param angleRad - The angle in radians
 * @returns The percentage value (0-100)
 */
export const angleToPercentage = (angleRad: number): number => {
  const angleDeg = (angleRad * 180) / Math.PI;
  const normalizedAngle =
    (angleDeg - GAUGE_ARC_START_ANGLE_DEG + GAUGE_FULL_ROTATION_DEG) %
    GAUGE_FULL_ROTATION_DEG;
  return Math.max(
    0,
    Math.min(
      PERCENT_SCALE,
      (normalizedAngle / GAUGE_ARC_SPAN_DEG) * PERCENT_SCALE,
    ),
  );
};

/**
 * Calculates the canvas coordinates for a given percentage.
 *
 * @param percentage - The percentage value (0-100)
 * @param width - Canvas width
 * @param height - Canvas height
 * @param radius - Gauge radius
 * @param arcRadius - Arc radius (typically radius * 0.85)
 * @returns Object with x and y coordinates
 */
export const getCoordinatesForPercentage = (
  percentage: number,
  width: number,
  height: number,
  radius: number,
  arcRadius: number,
): { x: number; y: number } => {
  const cx = width / 2;
  const cy = height / 2;
  const angle = percentageToAngle(percentage);
  return {
    x: cx + Math.cos(angle) * arcRadius,
    y: cy + Math.sin(angle) * arcRadius,
  };
};

/**
 * Calculates the distance between two points.
 *
 * @param x1 - First point x
 * @param y1 - First point y
 * @param x2 - Second point x
 * @param y2 - Second point y
 * @returns Distance between points
 */
export const distance = (
  x1: number,
  y1: number,
  x2: number,
  y2: number,
): number => {
  return Math.sqrt(Math.pow(x2 - x1, 2) + Math.pow(y2 - y1, 2));
};

/**
 * Formats power value in W or kW depending on magnitude.
 * Uses kW when the value is >= 1000W.
 */
export const formatPower = (watts: number): string => {
  if (watts >= WATT_TO_KILO_WATT_THRESHOLD)
    return `${(watts / WATT_TO_KILO_WATT_THRESHOLD).toFixed(2)} kW`;
  return `${Math.round(watts)} W`;
};

/**
 * Compute the angle (radians, 0-2π) of a mouse offset relative to gauge center.
 * Scale-invariant: produces the same angle regardless of rendered element size.
 */
export const gaugeAngleFromOffset = (
  offsetX: number,
  offsetY: number,
  elementWidth: number,
  elementHeight: number,
): number => {
  const cx = elementWidth / 2;
  const cy = elementHeight / 2;
  let angle = Math.atan2(offsetY - cy, offsetX - cx);
  if (angle < 0) angle += 2 * Math.PI;
  return angle;
};

/**
 * Formats an estimated time in minutes as ~HH:MM:SS.
 */
export const formatEstimatedTime = (minutes: number): string => {
  const totalSec = Math.floor(minutes * 60);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
};

/**
 * Formats a duration in milliseconds as HH:MM:SS.
 */
export const formatDuration = (ms: number): string => {
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
};

/**
 * Default electricity cost per kWh in pence.
 */
export const DefaultCostPerKwh = 24.83;

/**
 * Formats a pence amount as pounds.
 * Returns "£0.00", "£0.50", "£1.00", "£1.88", etc.
 */
const formatPence = (pence: number): string => {
  return `£${(pence / 100).toFixed(2)}`;
};

/**
 * Formats actual session cost. Returns "0p", "Xp", or "£X.XX".
 * Takes battery-side energy, converts to wall-side, multiplies by rate.
 */
export const formatCost = (
  energyAddedKwh: number | null,
  chargingEfficiency: number,
  costPerKwh: number,
): string => {
  if (energyAddedKwh == null || energyAddedKwh <= 0 || costPerKwh <= 0)
    return formatPence(0);
  const wallEnergyKwh = energyAddedKwh / chargingEfficiency;
  const costPence = Math.round(wallEnergyKwh * costPerKwh);
  return formatPence(costPence);
};

/**
 * Formats a pence amount (already in pence) as pounds: 14 → "£0.14", 1400 → "£14.00".
 * Use for backend-provided costs (e.g. a session's frozen costPence).
 */
export const formatPenceCost = (pence: number | null | undefined): string => {
  if (pence == null || pence < 0) return formatPence(0);
  return formatPence(Math.round(pence));
};

const minutesOfDay = (d: Date): number => d.getHours() * 60 + d.getMinutes();

const parseHHMM = (s: string): number | null => {
  const m = /^([01]\d|2[0-3]):([0-5]\d)$/.exec(s);
  return m ? Number(m[1]) * 60 + Number(m[2]) : null;
};

const withinWindow = (minutes: number, start: string, end: string): boolean => {
  const s = parseHHMM(start);
  const e = parseHHMM(end);
  if (s == null || e == null || s === e) return false;
  // [start, end); end <= start wraps past midnight.
  return s < e ? minutes >= s && minutes < e : minutes >= s || minutes < e;
};

/**
 * Returns the electricity rate (pence/kWh) in effect at `now`: the first matching
 * off-peak window's rate, else the base rate. Falls back to DefaultCostPerKwh when
 * no tariff is available. Used for live "actual"/"estimated" cost on the dashboard.
 */
export const activeRatePence = (
  tariff: TariffSettings | null | undefined,
  now: Date,
): number => {
  if (!tariff) return DefaultCostPerKwh;
  const minutes = minutesOfDay(now);
  for (const w of tariff.offPeakWindows) {
    if (withinWindow(minutes, w.start, w.end)) return w.ratePence;
  }
  return tariff.baseRatePence;
};

/**
 * Formats estimated total session cost.
 * Returns "£0.00", "£X.XX".
 * Computes from battery energy needed (current to target) converted to wall-side.
 */
export const formatEstimatedCost = (
  currentPercent: number,
  targetPercent: number,
  capacityKwh: number,
  chargingEfficiency: number,
  costPerKwh: number,
): string => {
  if (targetPercent <= currentPercent || capacityKwh <= 0 || costPerKwh <= 0)
    return formatPence(0);
  const batteryEnergyKwh =
    ((targetPercent - currentPercent) / 100) * capacityKwh;
  const wallEnergyKwh = batteryEnergyKwh / chargingEfficiency;
  const costPence = Math.round(wallEnergyKwh * costPerKwh);
  return formatPence(costPence);
};

/**
 * Formats estimated range at a given battery percentage.
 * Returns "0 mi", "X mi", or "X-Y mi".
 */
export const formatRange = (
  rangeMinMi: number,
  rangeMaxMi: number,
  percent: number,
): string => {
  const min = Math.round((rangeMinMi * percent) / 100);
  const max = Math.round((rangeMaxMi * percent) / 100);
  if (min === 0 && max === 0) return "0 mi";
  if (min === max) return `${min} mi`;
  return `${min}-${max} mi`;
};

/**
 * Checks whether an angle falls in the gap region of the gauge arc.
 * The arc spans from startAngle to startAngle + arcSpan. Angles outside
 * this range (wrapping around 2π) are in the gap.
 */
export const isAngleInGap = (
  angleRad: number,
  startAngle: number,
  arcSpan: number,
): boolean => {
  const delta =
    (((angleRad - startAngle) % (2 * Math.PI)) + 2 * Math.PI) % (2 * Math.PI);
  return delta > arcSpan;
};

/**
 * Converts a gauge angle (radians) to a percentage (0-100).
 * Handles gap angles by snapping to the nearest arc endpoint.
 */
export const gaugeAngleToPercentage = (
  angleRad: number,
  startAngle: number,
  arcSpan: number,
): number => {
  const delta =
    (((angleRad - startAngle) % (2 * Math.PI)) + 2 * Math.PI) % (2 * Math.PI);
  const absAngle = startAngle + delta;
  if (isAngleInGap(angleRad, startAngle, arcSpan)) {
    const distToStart = Math.abs(absAngle - startAngle);
    const distToEnd = Math.abs(absAngle - (startAngle + arcSpan));
    return distToStart <= distToEnd ? 0 : 100;
  }
  return (delta / arcSpan) * 100;
};

/**
 * Computes the shortest angular distance between two angles.
 */
export const angularDistance = (a: number, b: number): number => {
  const na = ((a % (2 * Math.PI)) + 2 * Math.PI) % (2 * Math.PI);
  const nb = ((b % (2 * Math.PI)) + 2 * Math.PI) % (2 * Math.PI);
  const d = Math.abs(na - nb);
  return Math.min(d, 2 * Math.PI - d);
};

/**
 * Formats voltage in volts.
 */
export const formatVoltage = (volts: number): string => {
  return `${Math.round(volts)} V`;
};

/**
 * Formats current in amperes.
 */
export const formatCurrent = (amps: number): string => {
  return `${amps.toFixed(2)} A`;
};

/**
 * Formats energy in kWh with 2 decimal places.
 */
export const formatEnergy = (kwh: number): string => {
  return `${kwh.toFixed(2)} kWh`;
};

/**
 * Formats a completion time as HH:MM:SS.
 */
export const formatCompletionTime = (
  etaMinutes: number | null,
  baseTimeMs: number,
): string => {
  if (etaMinutes == null || etaMinutes <= 0) return "-";
  const completion = new Date(baseTimeMs + etaMinutes * 60 * 1000);
  return completion.toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
};

/**
 * Calculates CO₂ saved in grams. When charging, uses actual energy added.
 * When idle, estimates from current to target SOC.
 */
export const getCo2SavedGrams = (
  status: string,
  energyAddedKwh: number | null,
  currentPercent: number,
  targetPercent: number,
  capacityKwh: number,
  chargingEfficiency: number,
  gridCarbonIntensity: number,
): number => {
  const petrolEmissionsPerKwh = 914;
  let energyKwh: number;

  if (status === "charging" && energyAddedKwh != null && energyAddedKwh > 0) {
    energyKwh = energyAddedKwh;
  } else if (
    capacityKwh > 0 &&
    chargingEfficiency > 0 &&
    targetPercent > currentPercent
  ) {
    energyKwh =
      (((targetPercent - currentPercent) / 100) * capacityKwh) /
      chargingEfficiency;
  } else {
    return 0;
  }

  return Math.max(
    0,
    Math.round(energyKwh * (petrolEmissionsPerKwh - gridCarbonIntensity)),
  );
};
