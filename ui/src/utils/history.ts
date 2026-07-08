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
      return "bg-gray-500";
  }
}

export function getStatusBadgeClass(status: string): string {
  switch (status) {
    case "completed":
      return "bg-emerald-500/20 text-emerald-400 border-emerald-500/30";
    case "cancelled":
      return "bg-red-500/20 text-red-400 border-red-500/30";
    case "pending":
      return "bg-yellow-500/20 text-yellow-400 border-yellow-500/30";
    case "active":
      return "bg-blue-500/20 text-blue-400 border-blue-500/30";
    case "conditioning":
      return "bg-amber-500/20 text-amber-400 border-amber-500/30";
    case "holding":
      return "bg-purple-500/20 text-purple-400 border-purple-500/30";
    default:
      return "bg-gray-500/20 text-gray-400 border-gray-500/30";
  }
}

export function getTotalEnergy(session: HistoryChargeSession): string {
  if (session.totalBatteryKwh !== undefined) {
    return session.totalBatteryKwh.toFixed(2);
  }
  return "-";
}
