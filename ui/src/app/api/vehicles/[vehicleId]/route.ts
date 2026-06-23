import { proxyRequest } from "@/lib/api-proxy";
import { isValidID } from "@/lib/validation";
import { NextRequest } from "next/server";

type RouteContext = { params: Promise<{ vehicleId: string }> };

export async function PATCH(req: NextRequest, { params }: RouteContext) {
  const { vehicleId } = await params;
  if (!isValidID(vehicleId)) {
    const { problemResponse } = await import("@/lib/problem-details");
    return problemResponse("Invalid vehicle ID format", 400);
  }
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    const { problemResponse } = await import("@/lib/problem-details");
    return problemResponse("Invalid request body", 400);
  }
  return proxyRequest({
    path: `/api/vehicles/${vehicleId}`,
    method: "PATCH",
    body,
  });
}

export async function DELETE(_req: NextRequest, { params }: RouteContext) {
  const { vehicleId } = await params;
  if (!isValidID(vehicleId)) {
    const { problemResponse } = await import("@/lib/problem-details");
    return problemResponse("Invalid vehicle ID format", 400);
  }
  return proxyRequest({ path: `/api/vehicles/${vehicleId}`, method: "DELETE" });
}
