"use client";

import type { CarbonIntensity, Vehicle } from "@/lib/schemas";

import { useElapsedEta, type ChargeStatus } from "@/hooks/useElapsedEta";
import {
  DefaultCostPerKwh,
  formatCompletionTime,
  formatCost,
  formatCurrent,
  formatDuration,
  formatEnergy,
  formatEstimatedCost,
  formatPower,
  formatRange,
  formatVoltage,
  getCo2SavedGrams,
} from "@/utils/gauge";

/**
 * Default UK grid carbon intensity in gCO₂/kWh when live API is unavailable.
 * Based on National Grid ESO 2024/2025 average (~140 gCO₂/kWh).
 * Fallback is conservative - actual grid intensity varies 100–150 depending
 * on wind output and time of day.
 */
const DefaultGridCarbonIntensity = 140;

const CarbonIntensityColors: Record<string, string> = {
  "very low": "text-success",
  low: "text-lime-700 dark:text-lime-400",
  moderate: "text-yellow-700 dark:text-yellow-400",
  high: "text-orange-700 dark:text-orange-400",
  "very high": "text-danger",
};

function getCarbonIntensityColor(index: string | undefined): string {
  return CarbonIntensityColors[index ?? ""] ?? "text-fg-muted";
}

interface StatsPanelProps {
  status: ChargeStatus;
  powerDraw: number;
  energyAddedKwh: number | null;
  voltage: number | null;
  current: number | null;
  errorMessage: string | null;
  sessionStartTime: number | null;
  startPercent: number;
  currentPercent: number;
  targetPercent: number;
  vehicle: Vehicle | null;
  carbonIntensity: CarbonIntensity | null;
  renderTimeMs?: number;
  /** Electricity rate (pence/kWh) for live cost; the rate in effect right now. */
  costPerKwh?: number;
}

export default function StatsPanel({
  status,
  powerDraw,
  energyAddedKwh,
  voltage,
  current,
  errorMessage,
  sessionStartTime,
  startPercent,
  currentPercent,
  targetPercent,
  vehicle,
  carbonIntensity,
  renderTimeMs,
  costPerKwh = DefaultCostPerKwh,
}: StatsPanelProps) {
  const { elapsed, totalTimeMin, baseTime, remainingSec } = useElapsedEta({
    status,
    sessionStartTime,
    currentPercent,
    targetPercent,
    vehicle,
    renderTimeMs,
    energyAddedKwh,
    startPercent,
  });

  const chargingEfficiency = vehicle?.chargingEfficiency ?? 0.8;
  const capacityKwh = vehicle?.capacityKwh ?? 0;
  const rangeMinMi = vehicle?.rangeMinMi ?? 0;
  const rangeMaxMi = vehicle?.rangeMaxMi ?? 0;
  const hasRange = rangeMinMi > 0 || rangeMaxMi > 0;
  const currentRange = formatRange(rangeMinMi, rangeMaxMi, currentPercent);
  const targetRange = formatRange(rangeMinMi, rangeMaxMi, targetPercent);

  const progressRange = targetPercent - startPercent;
  const progressPct =
    (status === "charging" ||
      status === "conditioning" ||
      status === "holding") &&
    progressRange > 0
      ? Math.max(
          0,
          Math.min(
            100,
            ((currentPercent - startPercent) / progressRange) * 100,
          ),
        )
      : 0;

  const energyRemainingKwh =
    capacityKwh > 0 && targetPercent > currentPercent
      ? Math.max(0, ((targetPercent - currentPercent) / 100) * capacityKwh)
      : 0;

  const actualCost = formatCost(energyAddedKwh, chargingEfficiency, costPerKwh);
  const estimatedCost = formatEstimatedCost(
    currentPercent,
    targetPercent,
    capacityKwh,
    chargingEfficiency,
    costPerKwh,
  );

  const gridCarbonIntensity =
    carbonIntensity?.actual ?? DefaultGridCarbonIntensity;
  const co2SavedGrams = getCo2SavedGrams(
    status,
    energyAddedKwh,
    currentPercent,
    targetPercent,
    capacityKwh,
    chargingEfficiency,
    gridCarbonIntensity,
  );

  const carbonIntensityColor = getCarbonIntensityColor(
    carbonIntensity?.index ?? "moderate",
  );

  return (
    <div className="grid grid-cols-3 gap-3 w-full">
      {/* Error */}
      {errorMessage && (
        <div className="col-span-3 rounded-xl bg-surface border border-border px-4 py-3">
          <div className="text-sm text-danger leading-snug">{errorMessage}</div>
        </div>
      )}

      {/* Row 1: Charge Duration | Time Remaining | Target Completion */}
      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Time elapsed since charging started"
        >
          Charge Duration
        </span>
        <div className="mt-1.5 text-2xl font-bold text-fg tabular-nums whitespace-nowrap">
          {formatDuration(elapsed)}
        </div>
      </div>

      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Estimated time until target battery level is reached, based on charger output and battery size"
        >
          Time Remaining
        </span>
        <div className="mt-1.5 text-2xl font-bold text-fg tabular-nums whitespace-nowrap">
          {remainingSec != null ? formatDuration(remainingSec * 1000) : "-"}
        </div>
      </div>

      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Predicted clock time when charging will finish"
        >
          Target Completion
        </span>
        {/* suppressHydrationWarning: toLocaleTimeString uses local timezone, server (UTC) ≠ browser timezone */}
        <div
          className="mt-1.5 text-2xl font-bold text-fg tabular-nums whitespace-nowrap"
          suppressHydrationWarning
        >
          {formatCompletionTime(totalTimeMin, baseTime)}
        </div>
      </div>

      {/* Row 2: Progress | Current Range | Target Range */}
      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="How far through the planned charge - from your start level to the target"
        >
          Progress
        </span>
        <div className="mt-1.5 text-2xl font-bold text-accent-muted tabular-nums whitespace-nowrap">
          {Math.floor(progressPct)}%
        </div>
      </div>

      {hasRange && (
        <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
          <span
            className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
            title="Estimated driving range at the battery's current level"
          >
            Current Range
          </span>
          <div className="mt-1.5 text-2xl font-bold text-danger whitespace-nowrap">
            {currentRange}
          </div>
        </div>
      )}

      {hasRange && (
        <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
          <span
            className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
            title="Estimated driving range when the target battery level is reached"
          >
            Target Range
          </span>
          <div className="mt-1.5 text-2xl font-bold text-orange-700 dark:text-orange-400 whitespace-nowrap">
            {targetRange}
          </div>
        </div>
      )}

      {/* Row 3: Power Draw | Current | Voltage */}
      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Electricity being drawn from the charger right now, in kilowatts"
        >
          Power Draw
        </span>
        <div className="mt-1.5 text-2xl font-bold text-warning tabular-nums whitespace-nowrap">
          {formatPower(powerDraw)}
        </div>
      </div>

      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Electrical current flowing from the charger right now, in amps"
        >
          Current
        </span>
        <div className="mt-1.5 text-2xl font-bold text-info tabular-nums whitespace-nowrap">
          {formatCurrent(current ?? 0)}
        </div>
      </div>

      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Electrical pressure from the charger right now, in volts"
        >
          Voltage
        </span>
        <div className="mt-1.5 text-2xl font-bold text-yellow-700 dark:text-yellow-400 tabular-nums whitespace-nowrap">
          {formatVoltage(voltage ?? 0)}
        </div>
      </div>

      {/* Row 4: Energy Added | Energy Left | CO₂ Saved */}
      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Electricity stored in the battery this session, in kilowatt-hours"
        >
          Energy Added
        </span>
        <div className="mt-1.5 text-2xl font-bold text-emerald-700 dark:text-emerald-400 tabular-nums whitespace-nowrap">
          {energyAddedKwh != null
            ? `${energyAddedKwh.toFixed(2)} kWh`
            : "0.00 kWh"}
        </div>
      </div>

      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Electricity still needed to reach the target battery level"
        >
          Energy Left
        </span>
        <div className="mt-1.5 text-2xl font-bold text-purple-700 dark:text-purple-400 tabular-nums whitespace-nowrap">
          {formatEnergy(energyRemainingKwh)}
        </div>
      </div>

      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Estimated CO₂ avoided compared to driving the same distance on petrol, based on current grid carbon intensity"
        >
          CO₂ Saved
        </span>
        <div className="mt-1.5 text-2xl font-bold text-lime-700 dark:text-lime-400 tabular-nums whitespace-nowrap">
          {co2SavedGrams > 0
            ? `${
                co2SavedGrams >= 1000
                  ? `${(co2SavedGrams / 1000).toFixed(1)} kg`
                  : `${co2SavedGrams} g`
              }`
            : "0 g"}
        </div>
      </div>

      {/* Row 5: Actual Cost | Estimated Cost | Carbon Intensity */}
      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Cost of electricity stored in the battery so far, based on your unit rate"
        >
          Actual Cost
        </span>
        <div className="mt-1.5 text-2xl font-bold text-rose-700 dark:text-rose-400 tabular-nums whitespace-nowrap">
          {actualCost}
        </div>
      </div>

      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Projected total cost to charge to the target level"
        >
          Estimated Cost
        </span>
        <div className="mt-1.5 text-2xl font-bold text-danger tabular-nums whitespace-nowrap">
          {estimatedCost}
        </div>
      </div>

      <div className="rounded-xl bg-surface border border-border px-4 py-3 text-center">
        <span
          className="text-[10px] font-semibold text-fg-muted uppercase tracking-[0.1em] whitespace-nowrap"
          title="Current grid CO₂ per kilowatt-hour - lower means greener electricity"
        >
          Carbon Intensity
        </span>
        <div
          className={`mt-1.5 text-2xl font-bold ${carbonIntensityColor} tabular-nums whitespace-nowrap`}
        >
          {gridCarbonIntensity}
          <span className="text-sm font-normal text-fg-muted ml-1">
            gCO₂/kWh
          </span>
        </div>
      </div>
    </div>
  );
}
