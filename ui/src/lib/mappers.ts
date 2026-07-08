export type SessionStatus =
  | "idle"
  | "charging"
  | "pending"
  | "conditioning"
  | "holding"
  | "error";

const IDLE_STATUSES = new Set(["completed", "inactive"]);

export function mapBackendStatus(backendStatus: string): SessionStatus {
  if (backendStatus === "active") return "charging";
  if (backendStatus === "pending") return "pending";
  if (backendStatus === "conditioning") return "conditioning";
  if (backendStatus === "holding") return "holding";
  if (backendStatus === "cancelled") return "error";
  if (IDLE_STATUSES.has(backendStatus)) return "idle";
  throw new Error(`Unknown backend status: ${backendStatus}`);
}
