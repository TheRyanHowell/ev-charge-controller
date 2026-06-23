"use client";

export function renderCarbonIntensityTooltip(value: number, timeLabel: string) {
  return (
    <div className="bg-gray-800 text-white text-xs rounded px-2 py-1 shadow-lg whitespace-nowrap">
      <span className="text-lime-400 font-semibold">
        {Math.round(value)} gCO₂/kWh
      </span>
      {" · "}
      <span className="text-gray-400">{timeLabel}</span>
    </div>
  );
}
