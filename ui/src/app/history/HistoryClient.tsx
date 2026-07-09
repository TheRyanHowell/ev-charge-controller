"use client";

import type { HistoryChargeSession, HistoryVehicle } from "@/lib/schemas";

import ConfirmDialog from "@/components/ConfirmDialog";
import ErrorBoundary from "@/components/ErrorBoundary";
import SessionDetail from "@/components/SessionDetail";
import { useHistory } from "@/hooks";
import { useTariff } from "@/hooks/useTariff";
import {
  activeRatePence,
  formatCost,
  formatPenceCost,
  formatRange,
} from "@/utils/gauge";
import {
  formatDuration,
  formatTimeRange,
  getStatusBadgeClass,
  getStatusColor,
  getTotalEnergy,
} from "@/utils/history";
import Link from "next/link";
import { useState } from "react";

interface HistoryClientProps {
  initialVehicles: HistoryVehicle[];
  initialSessions: HistoryChargeSession[];
  initialDate?: string;
}

export default function HistoryClient({
  initialVehicles,
  initialSessions,
  initialDate,
}: HistoryClientProps) {
  const {
    sessions,
    vehicles,
    selectedVehicleId,
    selectedDate,
    setSelectedDate,
    loading,
    error,
    hasMore,
    loadMore,
    handleVehicleChange,
    toggleExpand,
    deleteSession,
    deleteError,
    isExpanded,
  } = useHistory({
    initialVehicles,
    initialSessions,
    initialDate,
  });

  const [pendingDelete, setPendingDelete] =
    useState<HistoryChargeSession | null>(null);

  const handleDeleteRequest = (session: HistoryChargeSession) => {
    setPendingDelete(session);
  };

  const handleDeleteConfirm = async () => {
    if (!pendingDelete) return;
    // Only dismiss the dialog when the delete actually succeeded; a failed
    // delete keeps it open so the surfaced error is visible to the user.
    const ok = await deleteSession(pendingDelete.id);
    if (ok) {
      setPendingDelete(null);
    }
  };

  const handleDeleteCancel = () => {
    setPendingDelete(null);
  };

  if (loading) {
    return (
      <main className="min-h-screen bg-page-bg text-fg flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white mx-auto mb-4"></div>
          <p className="text-fg-muted">Loading...</p>
        </div>
      </main>
    );
  }

  return (
    <ErrorBoundary>
      <main className="min-h-screen bg-page-bg text-fg">
        <div className="w-full max-w-6xl mx-auto px-4 py-6 sm:px-6 sm:py-8">
          <header className="flex items-center justify-between mb-6">
            <Link
              href="/"
              className="text-fg-muted hover:text-fg transition-colors rounded-lg p-2 hover:bg-surface-raised"
              aria-label="Back to dashboard"
            >
              <i className="fas fa-home text-sm" aria-hidden="true"></i>
            </Link>
            <h1 className="text-xl font-semibold text-fg">Charge History</h1>
            <p className="text-xs text-fg-muted">
              {sessions.length} session{sessions.length !== 1 ? "s" : ""}
            </p>
          </header>

          <div className="flex items-center gap-3 mb-6">
            <input
              type="date"
              value={selectedDate}
              onChange={(e) => setSelectedDate(e.target.value)}
              aria-label="Filter by date"
              data-testid="date-picker"
              className="h-10 bg-surface border border-border rounded-lg px-3 text-sm text-fg-secondary focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <select
              value={selectedVehicleId ?? "all"}
              onChange={(e) => handleVehicleChange(e.target.value)}
              className="h-10 bg-surface border border-border rounded-lg px-3 text-sm text-fg-secondary focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="all">All Vehicles</option>
              {vehicles.map((v) => (
                <option key={v.id} value={v.id}>
                  {v.name}
                </option>
              ))}
            </select>
          </div>

          {error && (
            <div className="bg-red-900/50 border border-red-500/50 text-red-300 px-4 py-3 rounded-lg mb-4 text-sm">
              {error}
            </div>
          )}

          {sessions.length === 0 ? (
            <div className="text-center py-16">
              <div className="text-fg-muted text-lg mb-2">
                No charge sessions yet
              </div>
              <p className="text-fg-muted text-sm">
                Start charging to see your history
              </p>
            </div>
          ) : (
            <div>
              <div className="space-y-3">
                {sessions.map((session) => (
                  <SessionCard
                    key={session.id}
                    session={session}
                    vehicle={
                      vehicles.find((v) => v.id === session.vehicleId) ?? null
                    }
                    isExpanded={isExpanded(session.id)}
                    onToggle={() => toggleExpand(session.id)}
                    onDelete={() => handleDeleteRequest(session)}
                  />
                ))}
              </div>
              {hasMore && (
                <button
                  onClick={loadMore}
                  disabled={loading}
                  data-testid="load-more"
                  className="mt-4 w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-fg-secondary focus:outline-none focus:ring-2 focus:ring-blue-500 hover:bg-surface-hover disabled:opacity-50"
                >
                  {loading ? "Loading..." : "Load More"}
                </button>
              )}
            </div>
          )}
        </div>
      </main>

      {pendingDelete && (
        <ConfirmDialog
          title="Delete Session"
          message={
            deleteError
              ? "Couldn't delete this session. Please check your connection and try again."
              : `Delete this ${pendingDelete.status} charge session? This cannot be undone.`
          }
          onConfirm={handleDeleteConfirm}
          onCancel={handleDeleteCancel}
          confirmLabel="Delete"
          cancelLabel="Cancel"
          variant="danger"
        />
      )}
    </ErrorBoundary>
  );
}

/* ------------------------------------------------------------------ */
/*  Session Card                                                      */
/* ------------------------------------------------------------------ */

interface SessionCardProps {
  session: HistoryChargeSession;
  vehicle: HistoryVehicle | null;
  isExpanded: boolean;
  onToggle: () => void;
  onDelete: () => void;
}

function SessionCard({
  session,
  vehicle,
  isExpanded,
  onToggle,
  onDelete,
}: SessionCardProps) {
  const isDeletable =
    session.status === "completed" || session.status === "cancelled";
  const isActiveSession =
    session.status === "active" ||
    session.status === "conditioning" ||
    session.status === "holding";
  const vehicleName = vehicle?.name ?? session.vehicleId;
  const hasRange =
    vehicle != null && (vehicle.rangeMinMi > 0 || vehicle.rangeMaxMi > 0);
  const hasCo2 = session.avgCarbonIntensityGCo2PerKwh != null;

  // For active sessions use accumulated energy so far (defaulting to 0 if not yet available).
  const effectiveBatteryKwh =
    session.totalBatteryKwh ?? (isActiveSession ? 0 : null);
  const co2Grams =
    hasCo2 && effectiveBatteryKwh != null
      ? (effectiveBatteryKwh / (vehicle?.chargingEfficiency ?? 0.8)) *
        (session.avgCarbonIntensityGCo2PerKwh ?? 0)
      : null;
  const co2Label =
    co2Grams == null
      ? "-"
      : co2Grams >= 1000
        ? `${(co2Grams / 1000).toFixed(1)} kgCO₂`
        : `${Math.round(co2Grams)} gCO₂`;

  // For active sessions show targetPercent as the destination when endPercent isn't set yet.
  const displayEndPercent =
    session.endPercent ?? (isActiveSession ? session.targetPercent : null);
  const endLabel =
    isActiveSession && session.endPercent == null ? "Target" : "To";

  // For active sessions show 0.00 kWh when no energy has been recorded yet.
  const energyAdded = isActiveSession
    ? (session.totalBatteryKwh ?? 0).toFixed(2)
    : getTotalEnergy(session);

  // Completed sessions show the backend's frozen, tariff-accurate cost. Active
  // sessions have no frozen cost yet, so estimate from energy-so-far at the rate
  // currently in effect.
  const { settings: tariff } = useTariff();
  const costLabel =
    session.costPence != null
      ? formatPenceCost(session.costPence)
      : formatCost(
          effectiveBatteryKwh,
          vehicle?.chargingEfficiency ?? 0.8,
          activeRatePence(tariff, new Date()),
        );

  return (
    <div className="rounded-xl border border-border-subtle bg-surface-raised/80 overflow-hidden transition-all hover:border-border">
      {/* Collapsed view - clickable row */}
      <div className="flex items-stretch">
        <button
          onClick={onToggle}
          className="flex-1 text-left p-5 focus:outline-none"
          aria-expanded={isExpanded}
        >
          <div className="flex items-center gap-4">
            {/* Status dot */}
            <div className="flex-shrink-0">
              <div
                className={`w-2.5 h-2.5 rounded-full ${getStatusColor(session.status)} ${
                  session.status === "active" ? "animate-pulse" : ""
                }`}
              />
            </div>

            {/* Vehicle + date */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-medium text-sm truncate">
                  {vehicleName}
                </span>
                <span
                  className={`inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium border ${getStatusBadgeClass(
                    session.status,
                  )}`}
                >
                  {session.status}
                </span>
              </div>
              <div className="text-xs text-fg-muted mt-1">
                {new Date(session.createdAt).toLocaleDateString(undefined, {
                  month: "short",
                  day: "numeric",
                  year: "numeric",
                })}{" "}
                · {formatTimeRange(session.createdAt, session.endedAt)} ·{" "}
                {formatDuration(session.createdAt, session.endedAt)}
              </div>
            </div>

            {/* Stats - desktop (flex so each cell sizes to content, gap is always consistent) */}
            <div className="hidden sm:flex items-center gap-5 text-sm">
              <div className="text-right">
                <div className="text-fg-muted text-xs">From</div>
                <div className="text-fg-secondary">
                  {session.startPercent.toFixed(0)}%
                </div>
              </div>
              <div className="text-right">
                <div className="text-fg-muted text-xs">{endLabel}</div>
                <div className="text-fg-secondary">
                  {displayEndPercent?.toFixed(0) ?? "-"}%
                </div>
              </div>
              <div className="text-right">
                <div className="text-fg-muted text-xs">Added</div>
                <div className="text-emerald-400 font-medium whitespace-nowrap">
                  +{energyAdded} kWh
                </div>
              </div>
              <div className="text-right">
                <div className="text-fg-muted text-xs">Cost</div>
                <div className="text-danger font-medium whitespace-nowrap">
                  {costLabel}
                </div>
              </div>
              {hasRange && (
                <div className="text-right">
                  <div className="text-fg-muted text-xs">Range</div>
                  <div className="text-fg-secondary whitespace-nowrap">
                    {formatRange(
                      vehicle.rangeMinMi,
                      vehicle.rangeMaxMi,
                      session.endPercent ?? session.startPercent,
                    )}
                  </div>
                </div>
              )}
              {hasCo2 && (
                <div className="text-right">
                  <div className="text-fg-muted text-xs">CO₂</div>
                  <div className="text-lime-400 font-medium whitespace-nowrap">
                    {co2Label}
                  </div>
                </div>
              )}
            </div>

            {/* Chevron */}
            <i
              className={`fas fa-chevron-down w-4 h-4 text-fg-muted transition-transform duration-200 ${
                isExpanded ? "rotate-180" : ""
              }`}
              aria-hidden="true"
            ></i>
          </div>
        </button>

        {/* Delete button - sibling of card button to avoid nesting interactive elements */}
        {isDeletable && (
          <button
            type="button"
            onClick={onDelete}
            className="flex-shrink-0 self-center p-1.5 rounded-lg text-fg-muted hover:text-danger hover:bg-red-500/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500 transition-colors cursor-pointer"
            aria-label={`Delete ${session.status} session`}
            title="Delete session"
          >
            <i className="fas fa-trash-alt w-4 h-4" aria-hidden="true"></i>
          </button>
        )}
      </div>

      {/* Expanded detail */}
      {isExpanded && (
        <div className="border-t border-border-subtle px-5 pb-3">
          {/* Mobile stats - split into two rows to avoid 6-column cramping */}
          <div className="sm:hidden border-b border-border-subtle/50 py-3 mb-3 space-y-3">
            <div className="grid grid-cols-4 gap-3 text-center text-sm">
              <div>
                <div className="text-fg-muted text-xs">From</div>
                <div className="text-fg-secondary">
                  {session.startPercent.toFixed(0)}%
                </div>
              </div>
              <div>
                <div className="text-fg-muted text-xs">{endLabel}</div>
                <div className="text-fg-secondary">
                  {displayEndPercent?.toFixed(0) ?? "-"}%
                </div>
              </div>
              <div>
                <div className="text-fg-muted text-xs">Added</div>
                <div className="text-emerald-400 font-medium whitespace-nowrap">
                  +{energyAdded} kWh
                </div>
              </div>
              <div>
                <div className="text-fg-muted text-xs">Cost</div>
                <div className="text-danger font-medium whitespace-nowrap">
                  {costLabel}
                </div>
              </div>
            </div>
            {(hasRange || hasCo2) && (
              <div
                className={`grid gap-3 text-center text-sm ${
                  hasRange && hasCo2 ? "grid-cols-2" : "grid-cols-1"
                }`}
              >
                {hasRange && (
                  <div>
                    <div className="text-fg-muted text-xs">Range</div>
                    <div className="text-fg-secondary whitespace-nowrap">
                      {formatRange(
                        vehicle.rangeMinMi,
                        vehicle.rangeMaxMi,
                        session.endPercent ?? session.startPercent,
                      )}
                    </div>
                  </div>
                )}
                {hasCo2 && (
                  <div>
                    <div className="text-fg-muted text-xs">CO₂</div>
                    <div className="text-lime-400 font-medium whitespace-nowrap">
                      {co2Label}
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
          <SessionDetail sessionId={session.id} />
        </div>
      )}
    </div>
  );
}
