"use client";

import { polar } from "@/utils/gauge";

const VIEW_BOX = 300;
const CX = VIEW_BOX / 2;
const MAX_R = VIEW_BOX * 0.42;
const ARC_R = MAX_R * 0.85;
const PROGRESS_R = ARC_R;

interface GaugeInfoProps {
  currentPercent: number;
}

export function GaugeInfo({ currentPercent }: GaugeInfoProps) {
  const infoPos = polar(CX, CX, PROGRESS_R, currentPercent);

  return (
    <svg viewBox={`0 0 ${VIEW_BOX} ${VIEW_BOX}`} className="w-full h-full">
      <circle
        cx={infoPos.x}
        cy={infoPos.y}
        r="2"
        fill="rgba(255,255,255,0.15)"
        data-testid="gauge-info-current"
      />
    </svg>
  );
}
