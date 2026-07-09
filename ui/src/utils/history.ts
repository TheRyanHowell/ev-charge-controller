import type { HistoryChargeSession, HistoryVehicle } from "@/lib/schemas";

export function getVehicleName(
  vehicles: HistoryVehicle[],
  vehicleId: string,
): string {
  const vehicle = vehicles.find((v) => v.id === vehicleId);
  return vehicle?.name || vehicleId;
}

export function formatDuration(start: string, end?: string): string {
  if (!end) return "In progress";
  const diffMs = new Date(end).getTime() - new Date(start).getTime();
  const diffMins = Math.round(diffMs / 60000);
  if (diffMins < 60) return `${diffMins} min`;
  const hours = Math.floor(diffMins / 60);
  const mins = diffMins % 60;
  return `${hours}h ${mins}m`;
}

export function formatTimeRange(start: string, end?: string): string {
  const formatTime = (iso: string) => {
    const d = new Date(iso);
    return d.toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  };
  const startT = formatTime(start);
  if (!end) return `${startT} –`;
  return `${startT} – ${formatTime(end)}`;
}

export function getStatusColor(status: string): string {
  switch (status) {
    case "completed":
      return "bg-emerald-500";
    case "cancelled":
      return "bg-red-500";
    case "pending":
      return "bg-yellow-500";
    case "active":
      return "bg-blue-500";
    case "conditioning":
      return "bg-amber-500";
    case "holding":
      return "bg-purple-500";
    default:
      return "bg-fg-muted";
  }
}

export function getStatusBadgeClass(status: string): string {
  switch (status) {
    case "completed":
      return "bg-emerald-50 border-emerald-300 text-emerald-700 dark:bg-emerald-900/50 dark:border-emerald-500/50 dark:text-emerald-300";
    case "cancelled":
      return "bg-red-50 border-red-300 text-red-700 dark:bg-red-900/50 dark:border-red-500/50 dark:text-red-300";
    case "pending":
      return "bg-yellow-50 border-yellow-300 text-yellow-700 dark:bg-yellow-900/50 dark:border-yellow-500/50 dark:text-yellow-300";
    case "active":
      return "bg-blue-50 border-blue-300 text-blue-700 dark:bg-blue-900/50 dark:border-blue-500/50 dark:text-blue-300";
    case "conditioning":
      return "bg-amber-50 border-amber-300 text-amber-700 dark:bg-amber-900/50 dark:border-amber-500/50 dark:text-amber-300";
    case "holding":
      return "bg-purple-50 border-purple-300 text-purple-700 dark:bg-purple-900/50 dark:border-purple-500/50 dark:text-purple-300";
    default:
      return "bg-fg-muted/20 text-fg-muted border-fg-muted/30";
  }
}

export function getTotalEnergy(session: HistoryChargeSession): string {
  if (session.totalBatteryKwh !== undefined) {
    return session.totalBatteryKwh.toFixed(2);
  }
  return "-";
}
