import type { Vehicle } from "@/lib/schemas";

import { formatPower } from "@/utils/gauge";

interface StatusBarProps {
  tempError: string | null;
  selectedVehicle: Vehicle | null;
}

export default function StatusBar({
  tempError,
  selectedVehicle,
}: StatusBarProps) {
  return (
    <>
      {tempError && (
        <div
          className="mb-3 rounded-lg bg-red-900/30 border border-red-800/50 px-4 py-2.5 text-sm text-red-300"
          role="alert"
          aria-live="polite"
          aria-atomic="true"
        >
          {tempError}
        </div>
      )}

      {selectedVehicle && (
        <div className="flex flex-wrap items-center justify-center gap-2 mb-6">
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-surface border border-border text-xs text-gray-200">
            <i
              className="fas fa-charging-station text-gray-400 text-[10px]"
              aria-hidden="true"
            ></i>
            {selectedVehicle.name}
          </span>
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-surface border border-border text-xs text-gray-200">
            <i
              className="fas fa-battery-three-quarters text-gray-400 text-[10px]"
              aria-hidden="true"
            ></i>
            {selectedVehicle.capacityKwh} kWh
          </span>
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-surface border border-border text-xs text-gray-200">
            <i
              className="fas fa-bolt text-gray-400 text-[10px]"
              aria-hidden="true"
            ></i>
            {formatPower(selectedVehicle.chargerOutputW)}
          </span>
        </div>
      )}

      {!selectedVehicle && (
        <div className="text-center mt-6 text-gray-400">
          <p className="text-red-400/90 font-medium text-sm">
            No vehicle assigned to this plug &mdash; open settings to assign one
          </p>
        </div>
      )}
    </>
  );
}
