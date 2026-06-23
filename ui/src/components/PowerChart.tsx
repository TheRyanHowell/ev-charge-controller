"use client";

import { MIN_POINTS_FOR_RENDER, POLL_INTERVAL_MS } from "@/lib/constants";
import { PowerReadingSchema } from "@/lib/schemas";
import { PowerReading } from "@/types/chart";
import { useCallback } from "react";
import React from "react";

import LineChart from "./LineChart";
import { renderPowerTooltip } from "./PowerTooltip";

function PowerChart({
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
  const yExtractor = useCallback((r: PowerReading) => r.power, []);

  const timestampExtractor = useCallback((r: PowerReading) => {
    const d = new Date(r.timestamp);
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }, []);

  const computeYDomain = useCallback(
    (points: PowerReading[]): [number, number] => {
      if (points.length === 0) return [0, 1];
      let maxP = 0;
      for (const r of points) {
        if (r.power > maxP) maxP = r.power;
      }
      return [0, maxP];
    },
    [],
  );

  const yFormatter = useCallback((v: number) => {
    const kw = v / 1000;
    return kw < 10 ? kw.toFixed(2) : kw.toFixed(1);
  }, []);

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
      lineColor="#f97316"
      yFormatter={yFormatter}
      minPointsForRender={MIN_POINTS_FOR_RENDER}
      ariaLabel="Power draw chart"
      heightPx={160}
      messages={{
        loading: "Loading power data...",
        empty: sessionId
          ? "No power data for this session"
          : "No active charge session",
        waiting: "Not enough data to display",
      }}
      renderTooltipContent={renderPowerTooltip}
    />
  );
}

export default React.memo(PowerChart);
