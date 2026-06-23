"use client";

import type { Vehicle, VehicleModel } from "@/lib/schemas";

import Dialog from "@/components/Dialog";
import { apiGet, apiPost, apiDelete, apiPatchNoContent } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import { VehicleSchema, VehicleModelSchema } from "@/lib/schemas";
import { formatPenceCost } from "@/utils/gauge";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState, useCallback } from "react";

interface VehiclesClientProps {
  initialVehicles?: Vehicle[];
  initialModels?: VehicleModel[];
  initialError?: boolean;
  renderTimeMs: number;
}

export default function VehiclesClient({
  initialVehicles,
  initialModels,
  initialError,
  renderTimeMs,
}: VehiclesClientProps) {
  const queryClient = useQueryClient();
  const [showAdd, setShowAdd] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [editError, setEditError] = useState<string | null>(null);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

  const { data: vehicles = [], isError: vehiclesIsError } = useQuery({
    queryKey: queryKeys.vehicles.all,
    queryFn: () => apiGet("/api/vehicles", VehicleSchema),
    staleTime: 60 * 1000,
    // Skip initialData when SSR errored: lets React Query start in 'pending'
    // then transition to 'error' so the error UI renders correctly.
    initialData: initialError ? undefined : (initialVehicles ?? []),
    initialDataUpdatedAt: initialError ? undefined : renderTimeMs,
  });

  const { data: models = initialModels ?? [] } = useQuery({
    queryKey: queryKeys.vehicleModels.all,
    queryFn: () => apiGet("/api/vehicle-models", VehicleModelSchema),
    staleTime: 5 * 60 * 1000,
    initialData: initialModels,
    initialDataUpdatedAt: renderTimeMs,
  });

  const getModelName = useCallback((vehicle: Vehicle) => {
    if (vehicle.modelName && vehicle.modelName !== vehicle.name) {
      return ` (${vehicle.modelName})`;
    }
    return "";
  }, []);

  const createMutation = useMutation({
    mutationFn: (modelId: string) =>
      apiPost("/api/vehicles", VehicleSchema, { modelId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.vehicles.all });
      setShowAdd(false);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (vehicleId: string) => apiDelete(`/api/vehicles/${vehicleId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.vehicles.all });
      setDeleteConfirmId(null);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ vehicleId, name }: { vehicleId: string; name: string }) =>
      apiPatchNoContent(`/api/vehicles/${vehicleId}`, { name }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.vehicles.all });
      setEditingId(null);
      setEditError(null);
    },
    onError: (err) => {
      setEditError(err.message);
    },
  });

  const handleStartEdit = useCallback((v: Vehicle) => {
    setEditingId(v.id);
    setEditName(v.name);
    setEditError(null);
  }, []);

  const handleSaveEdit = useCallback(() => {
    if (editingId && editName.trim()) {
      updateMutation.mutate({ vehicleId: editingId, name: editName.trim() });
    }
  }, [editingId, editName, updateMutation]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter") handleSaveEdit();
      if (e.key === "Escape") setEditingId(null);
    },
    [handleSaveEdit],
  );

  return (
    <div className="min-h-screen bg-page-bg text-white">
      <div className="w-full max-w-6xl mx-auto px-4 py-6 sm:px-6 sm:py-8">
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-4">
            <Link
              href="/"
              className="text-gray-500 hover:text-gray-200 transition-colors rounded-lg p-2 hover:bg-surface-raised"
              aria-label="Back to dashboard"
            >
              <i className="fas fa-home text-sm" aria-hidden="true"></i>
            </Link>
          </div>
          <h1 className="text-xl font-semibold text-white">Vehicles</h1>
          <button
            type="button"
            onClick={() => setShowAdd(true)}
            disabled={createMutation.isPending}
            className="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
          >
            <i className="fa-solid fa-plus mr-1" /> Add vehicle
          </button>
        </div>

        {vehiclesIsError ? (
          <div className="text-center py-16" role="alert" aria-live="assertive">
            <p className="text-red-400 mb-2 font-medium">
              Failed to load vehicles
            </p>
            <p className="text-gray-400 text-sm">
              Something went wrong. Please refresh the page or try again.
            </p>
          </div>
        ) : (vehicles as Vehicle[]).length === 0 ? (
          <div className="text-center py-16">
            <p className="text-gray-400 mb-4">No vehicles yet</p>
            <button
              type="button"
              onClick={() => setShowAdd(true)}
              className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
            >
              Add your first vehicle
            </button>
          </div>
        ) : (
          <div className="space-y-2">
            {(vehicles as Vehicle[]).map((v) => (
              <div
                key={v.id}
                className="rounded-lg border border-gray-700 bg-gray-800/50 overflow-hidden"
              >
                <div className="flex items-center gap-3 px-4 py-3">
                  {editingId === v.id ? (
                    <input
                      type="text"
                      value={editName}
                      onChange={(e) => {
                        setEditName(e.target.value);
                        if (editError) setEditError(null);
                      }}
                      onKeyDown={handleKeyDown}
                      autoFocus
                      className="flex-1 rounded bg-gray-900 border border-gray-600 px-2 py-1 text-sm text-white focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                    />
                  ) : (
                    <Link
                      href={`/vehicles/${v.id}`}
                      className="flex-1 text-sm text-white hover:text-blue-400 transition-colors"
                    >
                      {v.name}
                      {getModelName(v)}
                    </Link>
                  )}

                  <span className="text-xs text-gray-400 shrink-0">
                    {v.capacityKwh} kWh
                  </span>

                  {editingId === v.id ? (
                    <>
                      <div className="flex gap-1 shrink-0">
                        <button
                          type="button"
                          onClick={handleSaveEdit}
                          disabled={
                            updateMutation.isPending || !editName.trim()
                          }
                          className="text-sm text-green-400 hover:text-green-300 disabled:opacity-50 rounded px-1 py-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-green-500"
                          title="Save"
                        >
                          <i className="fa-solid fa-check" />
                        </button>
                        <button
                          type="button"
                          onClick={() => setEditingId(null)}
                          className="text-sm text-gray-400 hover:text-white rounded px-1 py-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-gray-500"
                          title="Cancel"
                        >
                          <i className="fa-solid fa-xmark" />
                        </button>
                      </div>
                      {editError && (
                        <span className="text-xs text-amber-400 whitespace-nowrap">
                          {editError}
                        </span>
                      )}
                    </>
                  ) : (
                    <div className="flex gap-1 shrink-0">
                      <button
                        type="button"
                        onClick={() => handleStartEdit(v)}
                        className="text-sm text-gray-500 hover:text-blue-400 rounded px-1 py-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
                        title="Edit name"
                      >
                        <i className="fa-solid fa-pen" />
                      </button>
                      <button
                        type="button"
                        onClick={() => setDeleteConfirmId(v.id)}
                        className="text-sm text-gray-500 hover:text-red-400 rounded px-1 py-0.5 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-red-500"
                        title="Delete"
                      >
                        <i className="fa-solid fa-trash-can" />
                      </button>
                    </div>
                  )}
                </div>

                {(v.totalSessions ?? 0) > 0 && (
                  <div className="flex flex-wrap items-center gap-4 px-4 py-2 bg-gray-900/40 border-t border-gray-700/50 text-xs text-gray-400">
                    <span className="flex items-center gap-1">
                      <i
                        className="fas fa-plug-circle-bolt text-blue-400"
                        aria-hidden="true"
                      />
                      {v.totalSessions} session
                      {v.totalSessions !== 1 ? "s" : ""}
                    </span>
                    <span className="flex items-center gap-1">
                      <i
                        className="fas fa-bolt text-amber-400"
                        aria-hidden="true"
                      />
                      {(v.totalBatteryKwh ?? 0).toFixed(1)} kWh
                    </span>
                    <span className="flex items-center gap-1">
                      <i
                        className="fas fa-sterling-sign text-green-400"
                        aria-hidden="true"
                      />
                      {formatPenceCost(v.totalCostPence ?? 0)}
                    </span>
                    <span className="flex items-center gap-1">
                      <i
                        className="fas fa-coins text-green-300"
                        aria-hidden="true"
                      />
                      {formatPenceCost(
                        (v.totalCostPence ?? 0) / (v.totalSessions ?? 1),
                      )}{" "}
                      avg
                    </span>
                    {v.rangeMaxMi > 0 && (v.maxSessionBatteryKwh ?? 0) > 0 && (
                      <span className="flex items-center gap-1">
                        <i
                          className="fas fa-road text-sky-400"
                          aria-hidden="true"
                        />
                        {Math.round(
                          ((v.minSessionBatteryKwh ?? 0) / v.capacityKwh) *
                            v.rangeMaxMi,
                        )}
                        {" – "}
                        {Math.round(
                          ((v.maxSessionBatteryKwh ?? 0) / v.capacityKwh) *
                            v.rangeMaxMi,
                        )}{" "}
                        mi
                      </span>
                    )}
                    {(v.totalCo2Grams ?? 0) > 0 && (
                      <span className="flex items-center gap-1">
                        <i
                          className="fas fa-leaf text-gray-500"
                          aria-hidden="true"
                        />
                        {((v.totalCo2Grams ?? 0) / 1000).toFixed(2)} kg
                      </span>
                    )}
                    {v.lastSessionAt && (
                      <span className="flex items-center gap-1 ml-auto">
                        <i
                          className="fas fa-clock text-gray-500"
                          aria-hidden="true"
                        />
                        {formatRelativeTime(v.lastSessionAt)}
                      </span>
                    )}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Add vehicle dialog */}
      <Dialog isOpen={showAdd} onClose={() => setShowAdd(false)}>
        <div className="bg-gray-800 rounded-xl border border-gray-700 w-full max-w-md mx-4 p-5">
          <h2 className="text-base font-medium text-white mb-4">Add vehicle</h2>
          {(models as VehicleModel[]).length === 0 ? (
            <p className="text-sm text-gray-400">
              No vehicle models available.
            </p>
          ) : (
            <div className="space-y-1 max-h-60 overflow-y-auto">
              {(models as VehicleModel[]).map((m) => (
                <button
                  key={m.id}
                  type="button"
                  onClick={() => createMutation.mutate(m.id)}
                  disabled={createMutation.isPending}
                  className="w-full text-left rounded-lg px-3 py-2 text-sm text-white bg-gray-700/50 hover:bg-gray-700 disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
                >
                  <span>{m.name}</span>
                  <span className="text-gray-400 ml-2">
                    {m.capacityKwh} kWh
                  </span>
                </button>
              ))}
            </div>
          )}
          <div className="flex justify-end mt-4">
            <button
              type="button"
              onClick={() => setShowAdd(false)}
              className="rounded-lg bg-gray-700 px-3 py-1.5 text-sm text-gray-300 hover:bg-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      </Dialog>

      {/* Delete confirmation dialog */}
      <Dialog
        isOpen={deleteConfirmId !== null}
        onClose={() => setDeleteConfirmId(null)}
      >
        <div className="bg-gray-800 rounded-xl border border-gray-700 w-full max-w-sm mx-4 p-5">
          <h2 className="text-base font-medium text-white mb-2">
            Delete vehicle?
          </h2>
          <p className="text-sm text-gray-400 mb-4">
            This will remove the vehicle. Plugs assigned to it will be
            unassigned.
          </p>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => setDeleteConfirmId(null)}
              className="rounded-lg bg-gray-700 px-3 py-1.5 text-sm text-gray-300 hover:bg-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={() =>
                deleteConfirmId && deleteMutation.mutate(deleteConfirmId)
              }
              disabled={deleteMutation.isPending}
              className="rounded-lg bg-red-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-500 disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500 transition-colors"
            >
              Delete
            </button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}

function formatRelativeTime(isoString: string): string {
  const date = new Date(isoString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffMins < 1) return "just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}
