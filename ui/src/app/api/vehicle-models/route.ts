import { proxyGet } from "@/lib/api-proxy";

export async function GET() {
  return proxyGet({
    path: "/api/vehicle-models",
    detail: "Failed to fetch vehicle models",
  });
}
