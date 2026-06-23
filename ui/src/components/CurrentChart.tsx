"use client";

import { MIN_POINTS_FOR_RENDER, POLL_INTERVAL_MS } from "@/lib/constants";
import { PowerReadingSchema } from "@/lib/schemas";
import { PowerReading } from "@/types/chart";
import { useCallback } from "react";
import React from "react";

import { renderCurrentTooltip } from "./CurrentTooltip";
import LineChart from "./LineChart";

function CurrentChart({
  vehicleId,
  sessionId,
  shouldPoll,
  initialData,
}: {
  vehicleId?: string;
  sessionId?: string;
  shouldPoll?: boolean;
  initialData?: PowerReading[];
}) {
  const yExtractor = useCallback((r: PowerReading) => r.current, []);

  const timestampExtractor = useCallback((r: PowerReading) => {
    const d = new Date(r.timestamp);
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }, []);

  const computeYDomain = useCallback(
    (points: PowerReading[]): [number, number] => {
      if (points.length === 0) return [0, 1];
      let maxC = 0;
      for (const r of points) {
        if (r.current > maxC) maxC = r.current;
      }
      return [0, maxC];
    },
    [],
  );

  const yFormatter = useCallback((v: number) => v.toFixed(2), []);

  return (
    <LineChart<PowerReading>
      fetchConfig={{
        endpoint: sessionId
          ? `/api/power-readings?sessionId=${sessionId}`
          : "/api/power-readings",
        pollingIntervalMs: POLL_INTERVAL_MS,
      }}
      schema={PowerReadingSchema}
      vehicleId={vehicleId}
      initialData={initialData}
      shouldPoll={!sessionId && shouldPoll}
      yExtractor={yExtractor}
      timestampExtractor={timestampExtractor}
      yDomain={computeYDomain}
      lineColor="#3b82f6"
      yFormatter={yFormatter}
      minPointsForRender={MIN_POINTS_FOR_RENDER}
      ariaLabel="Current draw chart"
      heightPx={160}
      messages={{
        loading: "Loading current data...",
        empty: sessionId
          ? "No current data for this session"
          : "No active charge session",
        waiting: "Not enough data to display",
      }}
      renderTooltipContent={renderCurrentTooltip}
    />
  );
}

export default React.memo(CurrentChart);
