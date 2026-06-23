"use client";

import { formatSocPercent } from "@/utils/chartFormatters";

export function renderSocTooltip(value: number, timeLabel: string) {
  return (
    <div className="bg-gray-800 text-white text-xs rounded px-2 py-1 shadow-lg whitespace-nowrap">
      <span className="text-green-400 font-semibold">
        {formatSocPercent(value)}
      </span>
      {" · "}
      <span className="text-gray-400">{timeLabel}</span>
    </div>
  );
}
