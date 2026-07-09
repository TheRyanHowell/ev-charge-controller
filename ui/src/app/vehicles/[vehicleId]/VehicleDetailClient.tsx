"use client";

import type { Vehicle, VehicleStats } from "@/lib/schemas";

import CCVChart from "@/components/CCVChart";
import Dialog from "@/components/Dialog";
import { useFocusOnMount } from "@/hooks/useFocusOnMount";
import { apiDelete, apiGetSingle, apiPatchNoContent } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import { VehicleStatsSchema } from "@/lib/schemas";
import { formatPenceCost } from "@/utils/gauge";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

type TimeRange = "week" | "month" | "year" | "lifetime";

interface VehicleDetailClientProps {
  vehicleId: string;
  initialVehicle: Vehicle;
  initialStats: VehicleStats | null;
  renderTimeMs: number;
}

const TimeRanges: { value: TimeRange; label: string }[] = [
  { value: "week", label: "Week" },
  { value: "month", label: "Month" },
  { value: "year", label: "Year" },
  { value: "lifetime", label: "Lifetime" },
];

const CHART_TOOLTIP_STYLE = {
  backgroundColor: "var(--color-chart-tooltip-bg)",
  border: "1px solid var(--color-chart-grid)",
  borderRadius: "8px",
  color: "var(--color-fg)",
} as const;

function formatChartDate(label: unknown, long = false): string {
  if (typeof label !== "string") return String(label ?? "");
  const d = new Date(label + "T00:00:00");
  return d.toLocaleDateString(undefined, {
    ...(long && { weekday: "short", year: "numeric" }),
    month: "short",
    day: "numeric",
  });
}

export default function VehicleDetailClient({
  vehicleId,
  initialVehicle,
  initialStats,
  renderTimeMs,
}: VehicleDetailClientProps) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [timeRange, setTimeRange] = useState<TimeRange>("week");
  const [editing, setEditing] = useState(false);
  const [editName, setEditName] = useState(initialVehicle.name);
  const [editError, setEditError] = useState<string | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const focusEditInput = useFocusOnMount<HTMLInputElement>();

  const updateMutation = useMutation({
    mutationFn: () =>
      apiPatchNoContent(`/api/vehicles/${vehicleId}`, {
        name: editName.trim(),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.vehicles.all });
      setEditing(false);
      setEditError(null);
    },
    onError: (err) => {
      setEditError(err.message);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => apiDelete(`/api/vehicles/${vehicleId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.vehicles.all });
      router.push("/vehicles");
    },
  });

  const { data: stats = null } = useQuery({
    queryKey: [...queryKeys.vehicles.stats(vehicleId), timeRange] as const,
    queryFn: () =>
      apiGetSingle(
        `/api/vehicles/${vehicleId}/stats?range=${timeRange}`,
        VehicleStatsSchema,
      ),
    enabled: !!vehicleId,
    initialData: timeRange === "week" ? (initialStats ?? undefined) : undefined,
    initialDataUpdatedAt: timeRange === "week" ? renderTimeMs : undefined,
    placeholderData: (prev) => prev,
  });

  const hasData = stats && stats.totalSessions > 0;
  const modelName =
    initialVehicle.modelName && initialVehicle.modelName !== initialVehicle.name
      ? ` (${initialVehicle.modelName})`
      : "";

  // Derive wall energy from battery energy / efficiency
  const efficiency = initialVehicle.chargingEfficiency;
  const totalWallKwh = hasData ? stats.totalBatteryKwh / efficiency : 0;
  const avgSessionWallKwh = hasData ? stats.avgSessionKwh / efficiency : 0;
  const dailyEnergy = hasData
    ? stats.dailyEnergy.map((d) => ({
        ...d,
        wallKwh: d.batteryKwh / efficiency,
      }))
    : [];

  // Costs are the backend's frozen, tariff-accurate totals (time-weighted across
  // off-peak windows), not a flat-rate estimate.
  const totalCost = hasData ? formatPenceCost(stats.totalCostPence) : null;
  const avgCostPerSession = hasData
    ? formatPenceCost(stats.avgCostPence)
    : null;

  const hasRange =
    initialVehicle.rangeMaxMi > 0 &&
    hasData &&
    (stats.maxSessionBatteryKwh ?? 0) > 0;
  const minAddedRangeMi = hasRange
    ? Math.round(
        ((stats.minSessionBatteryKwh ?? 0) / initialVehicle.capacityKwh) *
          initialVehicle.rangeMaxMi,
      )
    : null;
  const maxAddedRangeMi = hasRange
    ? Math.round(
        ((stats.maxSessionBatteryKwh ?? 0) / initialVehicle.capacityKwh) *
          initialVehicle.rangeMaxMi,
      )
    : null;

  return (
    <div className="min-h-screen bg-page-bg text-fg">
      <div className="w-full max-w-6xl mx-auto px-4 py-6 sm:px-6 sm:py-8">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-4">
            <Link
              href="/vehicles"
              className="text-fg-muted hover:text-fg transition-colors rounded-lg p-2 hover:bg-surface-raised"
              aria-label="Back to vehicles"
            >
              <i className="fas fa-arrow-left text-sm" aria-hidden="true"></i>
            </Link>
          </div>
          {editing ? (
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={editName}
                onChange={(e) => {
                  setEditName(e.target.value);
                  if (editError) setEditError(null);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter") updateMutation.mutate();
                  if (e.key === "Escape") setEditing(false);
                }}
                ref={focusEditInput}
                className="rounded bg-surface-raised border border-border px-2 py-1 text-sm text-fg focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
              />
              <button
                type="button"
                onClick={() => updateMutation.mutate()}
                disabled={updateMutation.isPending || !editName.trim()}
                className="text-sm text-success hover:text-success disabled:opacity-50 rounded px-1 py-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-green-500"
                title="Save"
              >
                <i className="fa-solid fa-check" />
              </button>
              <button
                type="button"
                onClick={() => setEditing(false)}
                className="text-sm text-fg-muted hover:text-fg rounded px-1 py-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg-muted"
                title="Cancel"
              >
                <i className="fa-solid fa-xmark" />
              </button>
              {editError && (
                <span className="text-xs text-warning whitespace-nowrap">
                  {editError}
                </span>
              )}
            </div>
          ) : (
            <div className="flex items-center gap-3">
              <h1 className="text-xl font-semibold text-fg">
                {initialVehicle.name}
                {modelName}
              </h1>
              <button
                type="button"
                onClick={() => {
                  setEditName(initialVehicle.name);
                  setEditing(true);
                }}
                className="text-sm text-fg-muted hover:text-accent-muted rounded px-1 py-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
                title="Edit name"
              >
                <i className="fa-solid fa-pen" />
              </button>
              <button
                type="button"
                onClick={() => setDeleteConfirm(true)}
                className="text-sm text-fg-muted hover:text-danger rounded px-1 py-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-red-500"
                title="Delete"
              >
                <i className="fa-solid fa-trash-can" />
              </button>
            </div>
          )}
        </div>

        {/* Vehicle details */}
        <div className="mb-6 rounded-xl border border-border-subtle bg-surface-raised/80 px-3 py-4">
          <h2 className="text-sm font-medium text-fg-muted mb-3 uppercase tracking-wider">
            Vehicle Details
          </h2>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-x-4 gap-y-3 text-sm">
            {initialVehicle.modelName && (
              <DetailRow label="Model" value={initialVehicle.modelName} />
            )}
            <DetailRow
              label="Battery Capacity"
              value={`${initialVehicle.capacityKwh} kWh`}
            />
            <DetailRow
              label="Charger Output"
              value={`${(initialVehicle.chargerOutputW / 1000).toFixed(1)} kW`}
            />
            <DetailRow
              label="Charging Efficiency"
              value={`${(initialVehicle.chargingEfficiency * 100).toFixed(0)}%`}
            />
            {initialVehicle.rangeMinMi > 0 && (
              <DetailRow
                label="Range (min)"
                value={`${initialVehicle.rangeMinMi} mi`}
              />
            )}
            {initialVehicle.rangeMaxMi > 0 && (
              <DetailRow
                label="Range (max)"
                value={`${initialVehicle.rangeMaxMi} mi`}
              />
            )}
            {initialVehicle.packVoltageMaxV != null && (
              <DetailRow
                label="Pack Voltage Max"
                value={`${initialVehicle.packVoltageMaxV} V`}
              />
            )}
            {initialVehicle.packCutoffCurrentMa != null && (
              <DetailRow
                label="Pack Cutoff Current"
                value={`${(initialVehicle.packCutoffCurrentMa / 1000).toFixed(2)} A`}
              />
            )}
            {initialVehicle.time0to100Min != null && (
              <DetailRow
                label="0-100% Time"
                value={`${formatMinutes(initialVehicle.time0to100Min)}`}
              />
            )}
            {initialVehicle.time0to80Min != null && (
              <DetailRow
                label="0-80% Time"
                value={`${formatMinutes(initialVehicle.time0to80Min)}`}
              />
            )}
            {initialVehicle.time20to80Min != null && (
              <DetailRow
                label="20-80% Time"
                value={`${formatMinutes(initialVehicle.time20to80Min)}`}
              />
            )}
            {initialVehicle.time20to100Min != null && (
              <DetailRow
                label="20-100% Time"
                value={`${formatMinutes(initialVehicle.time20to100Min)}`}
              />
            )}
          </div>
        </div>

        {/* Time range filter */}
        <div className="flex items-center gap-2 mb-6">
          {TimeRanges.map((tr) => (
            <button
              key={tr.value}
              type="button"
              onClick={() => setTimeRange(tr.value)}
              className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 ${
                timeRange === tr.value
                  ? "bg-blue-600 text-fg"
                  : "bg-surface text-fg-muted hover:bg-surface-hover hover:text-fg"
              }`}
            >
              {tr.label}
            </button>
          ))}
        </div>

        {!hasData ? (
          <div className="text-center py-16">
            <p className="text-fg-muted mb-2">No charging data yet</p>
            <p className="text-fg-muted text-sm">
              Complete a charge session to see statistics
            </p>
          </div>
        ) : (
          <>
            {/* Stats cards */}
            <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
              <StatCard
                icon="fa-bolt"
                label="Total Energy"
                value={`${totalWallKwh.toFixed(1)} kWh`}
                color="text-warning"
              />
              <StatCard
                icon="fa-plug-circle-bolt"
                label="Sessions"
                value={stats.totalSessions.toString()}
                color="text-accent-muted"
              />
              <StatCard
                icon="fa-chart-simple"
                label="Avg per Session"
                value={`${avgSessionWallKwh.toFixed(2)} kWh`}
                color="text-purple-400"
              />
              <StatCard
                icon="fa-cloud"
                label="CO₂ Emissions"
                value={`${stats.totalCo2Grams > 0 ? formatCo2(stats.totalCo2Grams) : "-"}${stats.avgCarbonGCo2PerKwh != null ? ` (${stats.avgCarbonGCo2PerKwh.toFixed(0)} g/kWh)` : ""}`}
                color="text-fg-muted"
              />
              {totalCost && (
                <StatCard
                  icon="fa-sterling-sign"
                  label="Total Cost"
                  value={totalCost}
                  color="text-success"
                />
              )}
              {avgCostPerSession && (
                <StatCard
                  icon="fa-coins"
                  label="Avg Cost / Session"
                  value={avgCostPerSession}
                  color="text-success"
                />
              )}
              {hasRange && minAddedRangeMi !== null && (
                <StatCard
                  icon="fa-road"
                  label="Min Added Range"
                  value={`${minAddedRangeMi} mi`}
                  color="text-sky-400"
                />
              )}
              {hasRange && maxAddedRangeMi !== null && (
                <StatCard
                  icon="fa-road"
                  label="Max Added Range"
                  value={`${maxAddedRangeMi} mi`}
                  color="text-sky-300"
                />
              )}
            </div>

            {/* Energy chart */}
            <div className="rounded-xl border border-border-subtle bg-surface-raised/80 p-4">
              <h2 className="text-sm font-medium text-fg-muted mb-4 uppercase tracking-wider">
                Daily Energy
              </h2>
              <ResponsiveContainer width="100%" height={280}>
                <BarChart
                  data={dailyEnergy}
                  margin={{ top: 5, right: 5, left: -20, bottom: 5 }}
                >
                  <CartesianGrid
                    strokeDasharray="3 3"
                    stroke="var(--color-chart-grid)"
                  />
                  <XAxis
                    dataKey="date"
                    tick={{ fill: "var(--color-chart-label)", fontSize: 11 }}
                    tickFormatter={(val) => formatChartDate(val)}
                    tickLine={false}
                  />
                  <YAxis
                    tick={{ fill: "var(--color-chart-label)", fontSize: 11 }}
                    tickFormatter={(val: number) => `${val}`}
                    tickLine={false}
                    axisLine={false}
                  />
                  <Tooltip
                    contentStyle={CHART_TOOLTIP_STYLE}
                    labelFormatter={(label) => formatChartDate(label, true)}
                    formatter={(value, name) => {
                      if (value == null) return [String(value), String(name)];
                      if (typeof value !== "number") return [value, name];
                      if (name === "wallKwh")
                        return [`${value.toFixed(2)} kWh`, "Wall Energy"];
                      if (name === "sessionCount")
                        return [value.toString(), "Sessions"];
                      return [value, name];
                    }}
                  />
                  <Bar
                    dataKey="wallKwh"
                    radius={[4, 4, 0, 0]}
                    fill="#f59e0b"
                    name="wallKwh"
                  >
                    {dailyEnergy.map((entry, index) => (
                      <Cell
                        key={`cell-${index}`}
                        fill={entry.sessionCount > 1 ? "#d97706" : "#f59e0b"}
                      />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
              <div className="flex items-center justify-center gap-6 mt-3 text-xs text-fg-muted">
                <span className="flex items-center gap-1.5">
                  <span className="inline-block w-2.5 h-2.5 rounded-sm bg-warning" />
                  Wall Energy
                </span>
              </div>
            </div>

            {/* CO2 emissions chart */}
            {dailyEnergy.some((d) => d.co2Grams > 0) && (
              <div className="mt-6 rounded-xl border border-border-subtle bg-surface-raised/80 p-4">
                <h2 className="text-sm font-medium text-fg-muted mb-4 uppercase tracking-wider">
                  Daily CO₂ Emissions
                </h2>
                <ResponsiveContainer width="100%" height={220}>
                  <LineChart
                    data={dailyEnergy}
                    margin={{ top: 5, right: 5, left: -20, bottom: 5 }}
                  >
                    <CartesianGrid
                      strokeDasharray="3 3"
                      stroke="var(--color-chart-grid)"
                    />
                    <XAxis
                      dataKey="date"
                      tick={{ fill: "var(--color-chart-label)", fontSize: 11 }}
                      tickFormatter={(val) => formatChartDate(val)}
                      tickLine={false}
                    />
                    <YAxis
                      yAxisId="left"
                      tick={{ fill: "var(--color-chart-label)", fontSize: 11 }}
                      tickFormatter={(val: number) =>
                        val >= 1000 ? `${(val / 1000).toFixed(1)}kg` : `${val}g`
                      }
                      tickLine={false}
                      axisLine={false}
                    />
                    <YAxis
                      yAxisId="right"
                      tick={{ fill: "var(--color-chart-label)", fontSize: 11 }}
                      tickFormatter={(val: number) => `${val}`}
                      tickLine={false}
                      axisLine={false}
                    />
                    <Tooltip
                      contentStyle={CHART_TOOLTIP_STYLE}
                      labelFormatter={(label) => formatChartDate(label, true)}
                      formatter={(value, name) => {
                        if (value == null) return [String(value), String(name)];
                        if (typeof value !== "number") return [value, name];
                        if (name === "co2Grams")
                          return [formatCo2(value), "CO₂"];
                        if (name === "avgCarbonIntensityGCo2PerKwh")
                          return [`${value.toFixed(0)} g/kWh`, "Grid Carbon"];
                        return [value, name];
                      }}
                    />
                    <Line
                      yAxisId="left"
                      type="monotone"
                      dataKey="co2Grams"
                      stroke="var(--color-chart-label)"
                      strokeWidth={2}
                      dot={{ fill: "var(--color-chart-label)", r: 3 }}
                      activeDot={{ r: 5, fill: "var(--color-fg-muted)" }}
                      name="co2Grams"
                    />
                    <Line
                      yAxisId="right"
                      type="monotone"
                      dataKey="avgCarbonIntensityGCo2PerKwh"
                      stroke="var(--color-info)"
                      strokeWidth={1}
                      strokeDasharray="4 3"
                      opacity={0.4}
                      dot={{ fill: "var(--color-info)", r: 2 }}
                      activeDot={{ r: 4, fill: "#67e8f9" }}
                      name="avgCarbonIntensityGCo2PerKwh"
                    />
                  </LineChart>
                </ResponsiveContainer>
                <div className="flex items-center justify-center gap-6 mt-3 text-xs text-fg-muted">
                  <span className="flex items-center gap-1.5">
                    <span className="inline-block w-2.5 h-2.5 rounded-sm bg-fg-muted" />
                    CO₂ Emissions
                  </span>
                  <span className="flex items-center gap-1.5">
                    <span className="inline-block w-2.5 h-2.5 rounded-sm bg-info opacity-40" />
                    Grid Carbon Intensity
                  </span>
                </div>
              </div>
            )}
          </>
        )}

        {/* CC/CV Charging Profile */}
        <div className="mt-6 rounded-xl border border-border-subtle bg-surface-raised/80 p-4">
          <h2 className="text-sm font-medium text-fg-muted mb-4 uppercase tracking-wider">
            CC/CV Charging Profile
          </h2>
          <CCVChart vehicle={initialVehicle} />
        </div>

        {/* Delete confirmation dialog */}
        <Dialog isOpen={deleteConfirm} onClose={() => setDeleteConfirm(false)}>
          <div className="bg-surface rounded-xl border border-border w-full max-w-sm mx-4 p-5">
            <h2 className="text-base font-medium text-fg mb-2">
              Delete vehicle?
            </h2>
            <p className="text-sm text-fg-muted mb-4">
              This will remove the vehicle. Plugs assigned to it will be
              unassigned.
            </p>
            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setDeleteConfirm(false)}
                className="rounded-lg bg-surface px-3 py-1.5 text-sm text-fg-secondary hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => deleteMutation.mutate()}
                disabled={deleteMutation.isPending}
                className="rounded-lg bg-red-600 px-3 py-1.5 text-sm font-medium text-fg hover:bg-red-500 disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500 transition-colors"
              >
                Delete
              </button>
            </div>
          </div>
        </Dialog>
      </div>
    </div>
  );
}

function StatCard({
  icon,
  label,
  value,
  color,
}: {
  icon: string;
  label: string;
  value: string;
  color: string;
}) {
  return (
    <div className="rounded-xl border border-border-subtle bg-surface-raised/80 p-4">
      <div className="flex items-center gap-2 mb-2">
        <i className={`fas ${icon} text-xs ${color}`} aria-hidden="true"></i>
        <span className="text-xs text-fg-muted">{label}</span>
      </div>
      <div className={`text-lg font-semibold ${color}`}>{value}</div>
    </div>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs text-fg-muted">{label}</div>
      <div className="text-fg-secondary">{value}</div>
    </div>
  );
}

function formatMinutes(minutes: number): string {
  if (minutes === 0) return "-";
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (h === 0) return `${m}m`;
  if (m === 0) return `${h}h`;
  return `${h}h ${m}m`;
}

function formatCo2(grams: number): string {
  if (grams >= 1000) {
    return `${(grams / 1000).toFixed(2)} kg`;
  }
  return `${grams.toFixed(0)} g`;
}
