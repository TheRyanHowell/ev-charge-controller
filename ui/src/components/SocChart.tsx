"use client";

import { MIN_POINTS_FOR_RENDER, POLL_INTERVAL_MS } from "@/lib/constants";
import { SocSnapshotSchema } from "@/lib/schemas";
import { SOCSnapshot } from "@/types/chart";
import { useCallback } from "react";
import React from "react";

import LineChart from "./LineChart";
import { renderSocTooltip } from "./SocTooltip";

function SocChart({
  vehicleId,
  sessionId,
  shouldPoll,
  initialData,
}: {
  vehicleId?: string;
  sessionId?: string;
  shouldPoll?: boolean;
  initialData?: SOCSnapshot[];
}) {
  const yExtractor = useCallback((s: SOCSnapshot) => s.socPercent, []);

  const timestampExtractor = useCallback((s: SOCSnapshot) => {
    const d = new Date(s.timestamp);
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }, []);

  const yFormatter = useCallback((v: number) => String(Math.round(v)), []);

  return (
    <LineChart<SOCSnapshot>
      fetchConfig={{
        endpoint: sessionId
          ? `/api/soc-snapshots?sessionId=${sessionId}`
          : "/api/soc-snapshots",
        pollingIntervalMs: POLL_INTERVAL_MS,
      }}
      schema={SocSnapshotSchema}
      vehicleId={vehicleId}
      initialData={initialData}
      shouldPoll={!sessionId && shouldPoll}
      yExtractor={yExtractor}
      timestampExtractor={timestampExtractor}
      yDomain={[0, 100]}
      lineColor="#22c55e"
      yFormatter={yFormatter}
      minPointsForRender={MIN_POINTS_FOR_RENDER}
      ariaLabel="State of charge chart"
      heightPx={160}
      messages={{
        loading: "Loading SOC data...",
        empty: sessionId
          ? "No SOC data for this session"
          : "No active charge session",
        waiting: "Not enough data to display",
      }}
      renderTooltipContent={renderSocTooltip}
    />
  );
}

export default React.memo(SocChart);
