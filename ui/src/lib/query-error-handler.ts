import { ApiError } from "@/lib/api";

export function handleQueryError(error: unknown): void {
  if (error instanceof ApiError && error.status === 401) {
    window.location.href = "/login?reason=session-expired";
  }
}
