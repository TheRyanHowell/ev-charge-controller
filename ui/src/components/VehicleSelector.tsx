"use client";

import type { Vehicle, VehicleModel } from "@/lib/schemas";

import { useId } from "react";

export { parseVehicleSelectorValue } from "@/lib/vehicle-selector";

interface VehicleSelectorProps {
  label: string;
  vehicles: Vehicle[];
  models?: VehicleModel[];
  selectedVehicleId: string | null;
  onSelectVehicle: (vehicleId: string) => void;
  disabled?: boolean;
}

export default function VehicleSelector({
  label,
  vehicles,
  models,
  selectedVehicleId,
  onSelectVehicle,
  disabled,
}: VehicleSelectorProps) {
  const selectId = useId();

  // Models the user doesn't already have
  const usedModelIds = new Set(vehicles.map((v) => v.modelId).filter(Boolean));
  const availableModels = (models ?? []).filter((m) => !usedModelIds.has(m.id));

  return (
    <div>
      <label htmlFor={selectId} className="block text-xs text-fg-muted mb-1">
        {label}
      </label>
      <select
        id={selectId}
        value={selectedVehicleId ?? ""}
        onChange={(e) => onSelectVehicle(e.target.value)}
        disabled={disabled}
        className="w-full rounded-lg bg-surface-raised border border-border px-2.5 py-1.5 text-sm text-fg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        <option value="" disabled>
          Select a vehicle…
        </option>
        {vehicles.map((v) => {
          const modelName =
            v.modelName && v.modelName !== v.name ? ` (${v.modelName})` : "";
          return (
            <option key={v.id} value={v.id}>
              {v.name}
              {modelName} · {v.capacityKwh} kWh
            </option>
          );
        })}
        {availableModels.length > 0 && (
          <optgroup label="Add new model">
            {availableModels.map((m) => (
              <option key={`model-${m.id}`} value={`model:${m.id}`}>
                {m.name} · {m.capacityKwh} kWh
              </option>
            ))}
          </optgroup>
        )}
      </select>
    </div>
  );
}
