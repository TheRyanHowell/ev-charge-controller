"use client";

import { formatSocPercent } from "@/utils/chartFormatters";

export function renderSocTooltip(value: number, timeLabel: string) {
  return (
    <div className="bg-surface text-fg text-xs rounded px-2 py-1 shadow-lg whitespace-nowrap">
      <span className="text-success font-semibold">
        {formatSocPercent(value)}
      </span>
      {" · "}
      <span className="text-fg-muted">{timeLabel}</span>
    </div>
  );
}
