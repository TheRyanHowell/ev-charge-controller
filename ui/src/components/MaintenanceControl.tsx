import type { Plug } from "@/lib/schemas";

interface MaintenanceControlProps {
  /** The vehicle's 12V maintenance plug, or null when none is configured. */
  plug: Plug | null;
  onToggle?: () => void;
  isPending?: boolean;
  /** Opens the add-12V-charger flow; only used when no plug is configured. */
  onAdd12V?: () => void;
}

/**
 * Standalone 12V maintenance charger card for vehicles without a traction
 * battery (generic vehicles). Replaces the charge gauge on the dashboard.
 */
export default function MaintenanceControl({
  plug,
  onToggle,
  isPending,
  onAdd12V,
}: MaintenanceControlProps) {
  if (!plug) {
    return (
      <div className="rounded-xl border border-border-subtle bg-surface-raised/80 p-6 text-center">
        <p className="text-fg-muted mb-4">
          No 12V maintenance charger configured for this vehicle
        </p>
        {onAdd12V && (
          <button
            type="button"
            onClick={onAdd12V}
            className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 transition-colors"
          >
            <i className="fa-solid fa-plus mr-1" aria-hidden="true" /> Add 12V
            charger
          </button>
        )}
      </div>
    );
  }

  const toggleLabel = !plug.online
    ? "12V charger offline"
    : plug.powerOn
      ? "12V charger on - tap to turn off"
      : "12V charger off - tap to turn on";

  return (
    <div className="rounded-xl border border-border-subtle bg-surface-raised/80 p-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h2 className="text-sm font-medium text-fg-muted uppercase tracking-wider mb-1">
            12V Maintenance Charger
          </h2>
          <p className="text-fg">{plug.name}</p>
          <p
            className={`text-xs mt-1 ${plug.online ? "text-success" : "text-fg-muted"}`}
          >
            {plug.online ? "Online" : "Offline"}
          </p>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={plug.powerOn}
          aria-label={toggleLabel}
          onClick={onToggle}
          disabled={isPending}
          data-testid="maintenance-only-toggle"
          className={[
            "flex h-16 w-16 shrink-0 items-center justify-center rounded-full border-2 text-sm font-semibold transition-colors",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-500 disabled:opacity-50 disabled:cursor-not-allowed",
            !plug.online
              ? "border-border text-fg-muted"
              : plug.powerOn
                ? "border-cyan-500 bg-cyan-500/20 text-cyan-500"
                : "border-border text-fg-secondary hover:border-cyan-500",
          ].join(" ")}
        >
          12V
        </button>
      </div>
    </div>
  );
}
