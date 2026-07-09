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
          className="mb-3 rounded-lg bg-red-50 border border-red-200 px-4 py-2.5 text-sm text-red-700 dark:bg-red-900/30 dark:border-red-800/50 dark:text-red-300"
          role="alert"
          aria-live="polite"
          aria-atomic="true"
        >
          {tempError}
        </div>
      )}

      {selectedVehicle && (
        <div className="flex flex-wrap items-center justify-center gap-2 mb-6">
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-surface border border-border text-xs text-fg-secondary">
            <i
              className="fas fa-charging-station text-fg-muted text-[10px]"
              aria-hidden="true"
            ></i>
            {selectedVehicle.name}
          </span>
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-surface border border-border text-xs text-fg-secondary">
            <i
              className="fas fa-battery-three-quarters text-fg-muted text-[10px]"
              aria-hidden="true"
            ></i>
            {selectedVehicle.capacityKwh} kWh
          </span>
          <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-surface border border-border text-xs text-fg-secondary">
            <i
              className="fas fa-bolt text-fg-muted text-[10px]"
              aria-hidden="true"
            ></i>
            {formatPower(selectedVehicle.chargerOutputW)}
          </span>
        </div>
      )}

      {!selectedVehicle && (
        <div className="text-center mt-6 text-fg-muted">
          <p className="text-danger/90 font-medium text-sm">
            No vehicle assigned to this plug &mdash; open settings to assign one
          </p>
        </div>
      )}
    </>
  );
}
