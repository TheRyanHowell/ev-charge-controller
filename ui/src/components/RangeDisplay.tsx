"use client";

import { useGaugeStore } from "@/stores/gaugeStore";
import { formatRange } from "@/utils/gauge";

interface RangeDisplayProps {
  rangeMinMi: number;
  rangeMaxMi: number;
}

export default function RangeDisplay({
  rangeMinMi,
  rangeMaxMi,
}: RangeDisplayProps) {
  const currentPercent = useGaugeStore((s) => s.currentPercent);
  const targetPercent = useGaugeStore((s) => s.targetPercent);

  if (rangeMinMi === 0 && rangeMaxMi === 0) {
    return null;
  }

  const currentStr = formatRange(rangeMinMi, rangeMaxMi, currentPercent);
  const targetStr = formatRange(rangeMinMi, rangeMaxMi, targetPercent);

  const rangesMatch = currentStr === targetStr;

  return (
    <div className="mt-3 mb-6 text-center">
      <div className="flex justify-center text-xl font-semibold">
        <span className="inline-flex items-center gap-1">
          {currentPercent > 0 && !rangesMatch && (
            <span
              className="text-danger"
              aria-label={`Current range: ${currentStr}`}
            >
              {currentStr}
            </span>
          )}
          {currentPercent > 0 && !rangesMatch && (
            <>
              {" → "}
              <span
                className="text-orange-400"
                aria-label={`Target range: ${targetStr}`}
              >
                {targetStr}
              </span>
            </>
          )}
          {currentPercent > 0 && rangesMatch && (
            <span
              className="text-danger"
              aria-label={`Current range: ${currentStr}`}
            >
              {currentStr}
            </span>
          )}
          {currentPercent === 0 && (
            <span
              className="text-orange-400"
              aria-label={`Target range: ${targetStr}`}
            >
              {targetStr}
            </span>
          )}
        </span>
      </div>
    </div>
  );
}
