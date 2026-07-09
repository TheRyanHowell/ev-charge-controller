"use client";

import { GAUGE_ARC_SPAN_DEG } from "@/lib/constants";
import { polar } from "@/utils/gauge";

const VIEW_BOX = 300;
const CX = VIEW_BOX / 2;
const MAX_R = VIEW_BOX * 0.42;
const ARC_R = MAX_R * 0.85;
const PROGRESS_R = ARC_R;

function makeArcPath(fromPct: number, toPct: number, radius = PROGRESS_R) {
  const from = polar(CX, CX, radius, fromPct);
  const to = polar(CX, CX, radius, toPct);
  const span = Math.abs(toPct - fromPct);
  const largeArc = (span / 100) * GAUGE_ARC_SPAN_DEG > 180 ? 1 : 0;
  const sweep = toPct >= fromPct ? 1 : 0;
  return `M ${from.x} ${from.y} A ${radius} ${radius} 0 ${largeArc} ${sweep} ${to.x} ${to.y}`;
}

interface GaugeFaceProps {
  currentPercent: number;
  startPercent: number;
  status:
    | "idle"
    | "charging"
    | "pending"
    | "conditioning"
    | "holding"
    | "error";
}

export function GaugeFace({
  currentPercent,
  startPercent,
  status,
}: GaugeFaceProps) {
  const bgArcPath = makeArcPath(0, 100);

  return (
    <svg viewBox={`0 0 ${VIEW_BOX} ${VIEW_BOX}`} className="w-full h-full">
      <circle cx={CX} cy={CX} r={MAX_R} fill="var(--color-gauge-face)" />
      <circle
        cx={CX}
        cy={CX}
        r={MAX_R}
        fill="none"
        stroke="var(--color-gauge-ring)"
        strokeWidth="2"
      />
      <circle
        cx={CX}
        cy={CX}
        r={MAX_R - 1}
        fill="none"
        stroke="rgba(255,255,255,0.04)"
        strokeWidth="1"
      />
      <path
        d={bgArcPath}
        fill="none"
        stroke="var(--color-gauge-track)"
        strokeWidth="3"
        strokeLinecap="round"
      />

      <path
        d={makeArcPath(0, 20, PROGRESS_R + 1.5)}
        fill="none"
        stroke="rgba(239, 68, 68, 0.12)"
        strokeWidth="5"
        strokeLinecap="round"
      />
      <path
        d={makeArcPath(80, 100, PROGRESS_R + 1.5)}
        fill="none"
        stroke="rgba(217, 119, 6, 0.12)"
        strokeWidth="5"
        strokeLinecap="round"
      />

      {status === "charging" && (
        <path
          d={makeArcPath(startPercent, currentPercent)}
          fill="none"
          stroke="#22c55e"
          strokeWidth="3"
          strokeLinecap="round"
          opacity="0.9"
        />
      )}
    </svg>
  );
}
