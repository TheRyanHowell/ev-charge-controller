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
 * battery (generic vehicles). Replaces the charge gauge on the dashboard,
 * so it takes the gauge's place as the page's centred hero element.
 */
export default function MaintenanceControl({
  plug,
  onToggle,
  isPending,
  onAdd12V,
}: MaintenanceControlProps) {
  if (!plug) {
    return (
      <div className="w-full max-w-md mx-auto rounded-2xl border border-border-subtle bg-surface-raised/80 px-8 py-12 text-center">
        <div className="mx-auto mb-6 flex h-24 w-24 items-center justify-center rounded-full border-2 border-dashed border-border text-fg-muted">
          <i className="fa-solid fa-car-battery text-2xl" aria-hidden="true" />
        </div>
        <p className="text-fg mb-1 font-medium">No 12V maintenance charger</p>
        <p className="text-sm text-fg-muted mb-6">
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

  const statusDot = !plug.online
    ? "bg-warning"
    : plug.powerOn
      ? "bg-info"
      : "bg-fg-muted";

  const statusLine = !plug.online
    ? "Offline - check the plug's power and network"
    : plug.powerOn
      ? "Maintaining the 12V battery"
      : "Tap to start maintaining the 12V battery";

  return (
    <div className="w-full max-w-md mx-auto rounded-2xl border border-border-subtle bg-surface-raised/80 px-8 py-10 text-center">
      <p className="text-xs font-medium uppercase tracking-[0.2em] text-fg-muted">
        12V Maintenance Charger
      </p>
      <h2 className="mt-1 text-lg font-semibold text-fg">{plug.name}</h2>
      <p className="mt-1 inline-flex items-center gap-1.5 text-xs text-fg-muted">
        <span
          role="img"
          aria-label={plug.online ? "Online" : "Offline"}
          className={`h-1.5 w-1.5 rounded-full ${plug.online ? "bg-success" : "bg-fg-muted"}`}
        />
        {plug.online ? "Online" : "Offline"}
      </p>

      <button
        type="button"
        role="switch"
        aria-checked={plug.powerOn}
        aria-label={toggleLabel}
        onClick={onToggle}
        disabled={isPending}
        data-testid="maintenance-only-toggle"
        className={[
          "group mx-auto mt-8 flex h-36 w-36 items-center justify-center rounded-full border-2 transition-all duration-300",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-info focus-visible:ring-offset-2 focus-visible:ring-offset-surface-raised",
          "disabled:opacity-50 disabled:cursor-not-allowed",
          !plug.online
            ? "border-dashed border-border text-fg-muted"
            : plug.powerOn
              ? "border-info bg-info/10 text-info shadow-[0_0_60px_-12px] shadow-info"
              : "border-border text-fg-secondary hover:border-info hover:text-info",
        ].join(" ")}
      >
        <span className="flex flex-col items-center gap-1">
          <i
            className={`fa-solid fa-bolt text-2xl transition-transform duration-300 ${plug.online && plug.powerOn ? "" : "opacity-60"} group-hover:scale-110`}
            aria-hidden="true"
          />
          <span className="text-lg font-semibold tracking-wide">12V</span>
        </span>
      </button>

      <p className="mt-6 text-sm text-fg-secondary inline-flex items-center gap-2">
        <span className={`h-2 w-2 rounded-full ${statusDot}`} aria-hidden />
        {statusLine}
      </p>
    </div>
  );
}
