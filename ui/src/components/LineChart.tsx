"use client";

import { apiGet } from "@/lib/api";
import { DEFAULT_CHART_HEIGHT_PX } from "@/lib/constants";
import { queryKeys } from "@/lib/queryKeys";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo } from "react";
import {
  LineChart as RechartsLineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { z } from "zod";

import { ChartPlaceholder } from "./ChartPlaceholder";

export interface LineChartProps<T> {
  fetchConfig: { endpoint: string; pollingIntervalMs: number };
  /** Zod schema validating each item returned by the endpoint. */
  schema: z.ZodType<T>;
  vehicleId?: string;
  /** Pre-loaded data from server-side render. Immediately stale, will refetch. */
  initialData?: T[];
  /** Pre-loaded data for history (static). Will not refetch. */
  staticData?: T[];
  shouldPoll?: boolean;
  /** Optional filter applied to fetched/static data before rendering. */
  filterData?: (data: T[]) => T[];
  yExtractor: (item: T) => number;
  timestampExtractor: (item: T) => string;
  yDomain: [number, number] | ((data: T[]) => [number, number]);
  yFormatter?: (value: number, yMax: number) => string;
  lineColor: string;
  minPointsForRender: number;
  heightPx?: number;
  messages?: { loading: string; empty: string; waiting: string };
  ariaLabel: string;
  onSync?: (
    data: T[],
    loading: boolean,
    reason?: "no-data" | "few-points",
  ) => void;
  renderTooltipContent?: (value: number, timeLabel: string) => React.ReactNode;
}

export default function LineChart<T>({
  fetchConfig,
  schema,
  vehicleId,
  initialData,
  staticData,
  shouldPoll: shouldPollOpt = true,
  filterData,
  yExtractor,
  timestampExtractor,
  yDomain,
  yFormatter,
  lineColor,
  minPointsForRender,
  heightPx = DEFAULT_CHART_HEIGHT_PX,
  messages,
  ariaLabel,
  onSync,
  renderTooltipContent,
}: LineChartProps<T>) {
  const loadingMsg = messages?.loading ?? "Loading...";
  const emptyMsg = messages?.empty ?? "No active charge session";
  const waitingMsg = messages?.waiting ?? "Not enough data to display";

  const queryKey = useMemo(
    () =>
      queryKeys.lineChart.byEndpoint(fetchConfig.endpoint, vehicleId ?? null),
    [fetchConfig.endpoint, vehicleId],
  );

  const query = useQuery({
    queryKey,
    queryFn: async ({ signal }): Promise<T[]> => {
      const separator = fetchConfig.endpoint.includes("?") ? "&" : "?";
      const url = vehicleId
        ? `${fetchConfig.endpoint}${separator}vehicleId=${encodeURIComponent(vehicleId)}`
        : fetchConfig.endpoint;
      return apiGet(url, schema, { signal });
    },
    enabled: staticData === undefined,
    refetchInterval:
      shouldPollOpt && staticData === undefined
        ? fetchConfig.pollingIntervalMs
        : false,
    refetchOnWindowFocus: false,
    retry: false,
    initialData:
      initialData !== undefined
        ? initialData
        : staticData !== undefined
          ? staticData
          : undefined,
    initialDataUpdatedAt: initialData !== undefined ? 0 : undefined,
    staleTime: staticData !== undefined ? Infinity : 0,
  });

  const rawData = useMemo(() => query.data ?? [], [query.data]);
  const chartData = useMemo(
    () => (filterData ? filterData(rawData) : rawData),
    [filterData, rawData],
  );
  const loading = staticData === undefined && query.isPending;

  useEffect(() => {
    if (onSync) {
      if (chartData.length === 0 && !loading) {
        onSync(chartData, loading, "no-data");
      } else if (
        chartData.length > 0 &&
        chartData.length < minPointsForRender
      ) {
        onSync(chartData, loading, "few-points");
      } else {
        onSync(chartData, loading);
      }
    }
  }, [chartData, loading, onSync, minPointsForRender]);

  const computedYDomain = useMemo(() => {
    if (typeof yDomain === "function") {
      return yDomain(chartData);
    }
    return yDomain;
  }, [yDomain, chartData]);

  const [yMin, yMax] = computedYDomain;

  const formattedData = useMemo(() => {
    const points = chartData.map((item) => ({
      original: item,
      y: yExtractor(item),
      x: timestampExtractor(item),
    }));

    // With a single point Recharts renders a dot, not a line. Add a filler point
    // at the current time with the same value so a horizontal line is drawn.
    // The filler is dropped automatically once a second real point arrives.
    const first = points[0];
    if (points.length === 1 && first) {
      const nowLabel = new Date().toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      });
      points.push({ original: first.original, y: first.y, x: nowLabel });
    }

    return points;
  }, [chartData, yExtractor, timestampExtractor]);

  if (loading) {
    return <ChartPlaceholder message={loadingMsg} heightPx={heightPx} />;
  }

  if (chartData.length === 0) {
    return <ChartPlaceholder message={emptyMsg} heightPx={heightPx} />;
  }

  if (chartData.length < minPointsForRender) {
    return <ChartPlaceholder message={waitingMsg} heightPx={heightPx} />;
  }

  return (
    <div
      className="mt-1 sm:mt-2 rounded-lg overflow-hidden w-full flex flex-col"
      role="img"
      aria-label={ariaLabel}
    >
      <ResponsiveContainer width="100%" height={heightPx}>
        <RechartsLineChart
          data={formattedData}
          margin={{ top: 16, right: 16, bottom: 12, left: 16 }}
        >
          <XAxis
            dataKey="x"
            tick={{ fontSize: 10, fill: "#6b7280" }}
            height={20}
          />
          <YAxis
            domain={[yMin, yMax]}
            tick={{ fontSize: 10, fill: "#6b7280" }}
            tickFormatter={(v: number) =>
              yFormatter ? yFormatter(v, yMax) : String(Math.round(v))
            }
            width={40}
          />
          <Tooltip
            contentStyle={{
              backgroundColor: "#1f2937",
              border: "none",
              borderRadius: "8px",
              padding: "8px 12px",
            }}
            content={({ active, payload }) => {
              if (!active || !payload || payload.length === 0) return null;
              const dataPoint = payload[0];
              if (!dataPoint) return null;
              const y = dataPoint.value as number;
              const x = dataPoint.payload.x as string;
              return renderTooltipContent ? (
                renderTooltipContent(y, x)
              ) : (
                <div className="text-xs text-fg">
                  {Math.round(y)}, {x}
                </div>
              );
            }}
          />
          <Line
            type="monotone"
            dataKey="y"
            stroke={lineColor}
            dot={false}
            isAnimationActive={false}
          />
        </RechartsLineChart>
      </ResponsiveContainer>
    </div>
  );
}
