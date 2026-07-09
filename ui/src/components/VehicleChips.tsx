"use client";

import type { Plug, Vehicle } from "@/lib/schemas";

interface VehicleChipsProps {
  vehicles: Vehicle[];
  plugs: Plug[];
  selectedVehicleId: string | null;
  onSelect: (vehicleId: string) => void;
}

export default function VehicleChips({
  vehicles,
  plugs,
  selectedVehicleId,
  onSelect,
}: VehicleChipsProps) {
  if (vehicles.length === 0) return null;

  return (
    <div className="flex items-center gap-2 mb-4 overflow-x-auto pb-1">
      {vehicles.map((vehicle) => {
        const chargingPlug = plugs.find(
          (p) => p.type === "charging" && p.vehicleId === vehicle.id,
        );
        const isOnline = chargingPlug?.online ?? false;
        const isSelected = selectedVehicleId === vehicle.id;

        return (
          <button
            key={vehicle.id}
            onClick={() => onSelect(vehicle.id)}
            className={[
              "flex items-center gap-2 px-3 py-1.5 rounded-full text-sm font-medium whitespace-nowrap transition-colors",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500",
              isSelected
                ? "bg-fg text-page-bg"
                : "bg-surface border border-border text-fg-secondary hover:bg-surface-hover hover:text-fg",
            ].join(" ")}
            aria-pressed={isSelected}
          >
            <span
              role="img"
              className={[
                "h-2 w-2 rounded-full flex-shrink-0",
                isOnline ? "bg-success" : "bg-fg-muted",
              ].join(" ")}
              aria-label={isOnline ? "Online" : "Offline"}
            />
            {vehicle.name}
          </button>
        );
      })}
    </div>
  );
}
