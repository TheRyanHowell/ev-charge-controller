"use client";

export function renderCurrentTooltip(value: number, timeLabel: string) {
  return (
    <div className="bg-gray-800 text-white text-xs rounded px-2 py-1 shadow-lg whitespace-nowrap">
      <span className="text-blue-400 font-semibold">{value.toFixed(2)} A</span>
      {" · "}
      <span className="text-gray-400">{timeLabel}</span>
    </div>
  );
}
