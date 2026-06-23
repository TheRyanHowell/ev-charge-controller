"use client";

import { MIN_POINTS_FOR_RENDER, POLL_INTERVAL_MS } from "@/lib/constants";
import { PowerReadingSchema } from "@/lib/schemas";
import { PowerReading } from "@/types/chart";
import { useCallback } from "react";
import React from "react";

import { renderCarbonIntensityTooltip } from "./CarbonIntensityTooltip";
import LineChart from "./LineChart";

function CarbonIntensityChart({
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
  const filterData = useCallback(
    (readings: PowerReading[]) =>
      readings.filter((r) => r.carbonIntensityGCo2PerKwh != null),
    [],
  );

  const yExtractor = useCallback(
    (r: PowerReading) => r.carbonIntensityGCo2PerKwh ?? 0,
    [],
  );

  const timestampExtractor = useCallback((r: PowerReading) => {
    const d = new Date(r.timestamp);
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }, []);

  const computeYDomain = useCallback(
    (points: PowerReading[]): [number, number] => {
      if (points.length === 0) return [0, 1];
      let maxCI = 0;
      for (const r of points) {
        const v = r.carbonIntensityGCo2PerKwh ?? 0;
        if (v > maxCI) maxCI = v;
      }
      return [0, maxCI];
    },
    [],
  );

  const yFormatter = useCallback((v: number) => String(Math.round(v)), []);

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
      filterData={filterData}
      yExtractor={yExtractor}
      timestampExtractor={timestampExtractor}
      yDomain={computeYDomain}
      lineColor="#84cc16"
      yFormatter={yFormatter}
      minPointsForRender={MIN_POINTS_FOR_RENDER}
      ariaLabel="Carbon intensity chart"
      heightPx={160}
      messages={{
        loading: "Loading carbon data...",
        empty: "No carbon data",
        waiting: "Not enough data to display",
      }}
      renderTooltipContent={renderCarbonIntensityTooltip}
    />
  );
}

export default React.memo(CarbonIntensityChart);
