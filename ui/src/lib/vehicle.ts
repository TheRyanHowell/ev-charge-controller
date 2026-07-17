/**
 * A vehicle or catalog model with zero battery capacity (the "generic" entry,
 * e.g. a petrol vehicle) has no traction battery: it supports only 12V
 * maintenance charging, never EV charge sessions.
 */
export function hasBattery(v: { capacityKwh: number }): boolean {
  return v.capacityKwh > 0;
}
