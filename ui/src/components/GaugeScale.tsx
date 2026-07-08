"use client";

import { polar } from "@/utils/gauge";

const VIEW_BOX = 300;
const CX = VIEW_BOX / 2;
const MAX_R = VIEW_BOX * 0.42;
const ARC_R = MAX_R * 0.85;
const PROGRESS_R = ARC_R;

interface GaugeScaleProps {
  targetPercent: number;
  status:
    | "idle"
    | "charging"
    | "pending"
    | "conditioning"
    | "holding"
    | "error";
  startPercent?: number;
}

export function GaugeScale({
  targetPercent,
  status,
  startPercent = 0,
}: GaugeScaleProps) {
  const tickLabels = [0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100];

  return (
    <svg viewBox={`0 0 ${VIEW_BOX} ${VIEW_BOX}`} className="w-full h-full">
      {tickLabels.map((label) => {
        const isMajor = label % 20 === 0;
        const tickLen = isMajor ? 14 : 8;
        const tickOuter = polar(CX, CX, PROGRESS_R + tickLen / 2, label);
        const tickInner = polar(CX, CX, PROGRESS_R - tickLen / 2, label);
        const labelR = PROGRESS_R + (isMajor ? 28 : 20);
        const labelPos = polar(CX, CX, labelR, label);
        return (
          <g key={label}>
            <line
              x1={tickOuter.x}
              y1={tickOuter.y}
              x2={tickInner.x}
              y2={tickInner.y}
              stroke={
                isMajor ? "rgba(255,255,255,0.8)" : "rgba(255,255,255,0.35)"
              }
              strokeWidth={isMajor ? 2 : 1.5}
              strokeLinecap="round"
            />
            <text
              x={labelPos.x}
              y={labelPos.y}
              fill={
                isMajor ? "rgba(255,255,255,0.9)" : "rgba(255,255,255,0.45)"
              }
              fontSize={isMajor ? "9" : "8"}
              textAnchor="middle"
              dominantBaseline="central"
            >
              {label}
            </text>
          </g>
        );
      })}

      {status === "charging" && (
        <circle
          cx={polar(CX, CX, PROGRESS_R, startPercent).x}
          cy={polar(CX, CX, PROGRESS_R, startPercent).y}
          r="4"
          fill="#22c55e"
        />
      )}

      {(() => {
        const t = polar(CX, CX, PROGRESS_R, targetPercent);
        const tInner = polar(CX, CX, PROGRESS_R - 8, targetPercent);
        return (
          <g>
            <line
              x1={t.x}
              y1={t.y}
              x2={tInner.x}
              y2={tInner.y}
              stroke="#f97316"
              strokeWidth="2"
              strokeLinecap="round"
            />
            <circle cx={t.x} cy={t.y} r="3" fill="#f97316" />
          </g>
        );
      })()}
    </svg>
  );
}
