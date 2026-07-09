"use client";

import { WATT_TO_KILO_WATT_THRESHOLD } from "@/lib/constants";

export function renderPowerTooltip(value: number, timeLabel: string) {
  const formatted =
    value >= WATT_TO_KILO_WATT_THRESHOLD
      ? `${(value / WATT_TO_KILO_WATT_THRESHOLD).toFixed(2)} kW`
      : `${Math.round(value)} W`;

  return (
    <div className="bg-surface text-fg text-xs rounded px-2 py-1 shadow-lg whitespace-nowrap">
      <span className="text-orange-400 font-semibold">{formatted}</span>
      {" · "}
      <span className="text-fg-muted">{timeLabel}</span>
    </div>
  );
}
