"use client";

import { polar } from "@/utils/gauge";

const VIEW_BOX = 300;
const CX = VIEW_BOX / 2;
const MAX_R = VIEW_BOX * 0.42;
const ARC_R = MAX_R * 0.85;
const PROGRESS_R = ARC_R;

interface GaugeNeedleProps {
  currentPercent: number;
}

export function GaugeNeedle({ currentPercent }: GaugeNeedleProps) {
  const needleTip = polar(CX, CX, PROGRESS_R - 12, currentPercent);

  return (
    <svg viewBox={`0 0 ${VIEW_BOX} ${VIEW_BOX}`} className="w-full h-full">
      <circle
        cx={CX}
        cy={CX}
        r="10"
        fill="var(--color-gauge-ring)"
        stroke="var(--color-gauge-hub)"
        strokeWidth="1"
      />
      <circle cx={CX} cy={CX} r="4" fill="var(--color-gauge-hub)" />
      <line
        x1={CX}
        y1={CX}
        x2={needleTip.x}
        y2={needleTip.y}
        stroke="#ef4444"
        strokeWidth="3"
        strokeLinecap="round"
      />
      <circle cx={needleTip.x} cy={needleTip.y} r="3" fill="#ef4444" />
    </svg>
  );
}
