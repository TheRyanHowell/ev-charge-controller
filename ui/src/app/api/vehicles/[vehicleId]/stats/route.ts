import { proxyGet } from "@/lib/api-proxy";
import { isValidID } from "@/lib/validation";
import { NextRequest } from "next/server";

type RouteContext = { params: Promise<{ vehicleId: string }> };

export async function GET(req: NextRequest, { params }: RouteContext) {
  const { vehicleId } = await params;
  if (!isValidID(vehicleId)) {
    const { problemResponse } = await import("@/lib/problem-details");
    return problemResponse("Invalid vehicle ID format", 400);
  }
  const range = req.nextUrl.searchParams.get("range") ?? "lifetime";
  return proxyGet({
    path: `/api/vehicles/${vehicleId}/stats?range=${range}`,
    detail: "Failed to fetch vehicle stats",
  });
}
