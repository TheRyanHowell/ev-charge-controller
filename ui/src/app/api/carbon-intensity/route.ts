import { proxyGet } from "@/lib/api-proxy";

export async function GET() {
  return proxyGet({
    path: "/api/carbon-intensity",
    detail: "Failed to fetch carbon intensity",
  });
}
