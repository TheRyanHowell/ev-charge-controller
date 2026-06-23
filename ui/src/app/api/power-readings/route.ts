import { proxyGet } from "@/lib/api-proxy";
import { validateSearchParams } from "@/lib/query-params";

export async function GET(request: { url: string }) {
  const url = new URL(request.url);
  const validated = validateSearchParams(
    url.searchParams,
    new Set(["sessionId", "vehicleId"]),
  );
  return proxyGet({
    path: "/api/power-readings",
    searchParams: validated,
    detail: "Failed to fetch power readings",
  });
}
