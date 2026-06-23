/**
 * Snaps a percentage value to the nearest increment.
 * If within 2.5% of a 10% mark, snaps to 10%.
 * Otherwise, snaps to the nearest 5% increment.
 *
 * @param value - The percentage value to snap (0-100)
 * @returns The snapped percentage value
 *
 * @example
 * snapPercentage(4)   // returns 5
 * snapPercentage(7)   // returns 5
 * snapPercentage(12)  // returns 10
 * snapPercentage(13)  // returns 15
 * snapPercentage(18)  // returns 20
 * snapPercentage(23)  // returns 25
 */
export const snapPercentage = (value: number): number => {
  const clamped = Math.max(0, Math.min(100, value));
  const nearest10 = Math.round(clamped / 10) * 10;
  return Math.abs(clamped - nearest10) <= 2.5
    ? nearest10
    : Math.round(clamped / 5) * 5;
};

/**
 * Snaps a percentage value to 1% increments (fine-grained control).
 *
 * @param value - The percentage value to snap (0-100)
 * @returns The snapped percentage value (rounded to nearest 1%)
 */
export const snapToPercent = (value: number): number => {
  return Math.round(value);
};
