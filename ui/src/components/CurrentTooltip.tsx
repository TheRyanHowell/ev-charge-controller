"use client";

export function renderCurrentTooltip(value: number, timeLabel: string) {
  return (
    <div className="bg-surface text-fg text-xs rounded px-2 py-1 shadow-lg whitespace-nowrap">
      <span className="text-accent-muted font-semibold">
        {value.toFixed(2)} A
      </span>
      {" · "}
      <span className="text-fg-muted">{timeLabel}</span>
    </div>
  );
}
