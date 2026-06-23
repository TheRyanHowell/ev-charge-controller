import { WATT_TO_KILO_WATT_THRESHOLD } from "@/lib/constants";

export function formatPower(watts: number, maxWatts: number): string {
  if (maxWatts >= WATT_TO_KILO_WATT_THRESHOLD) {
    return `${(watts / WATT_TO_KILO_WATT_THRESHOLD).toFixed(2)} kW`;
  }
  return `${Math.round(watts)} W`;
}

export function formatSocPercent(value: number): string {
  return `${value.toFixed(2)}%`;
}
