import { proxyRequest } from "@/lib/api-proxy";
import { isValidID } from "@/lib/validation";
import { NextRequest } from "next/server";

type RouteContext = { params: Promise<{ id: string }> };

export async function DELETE(_req: NextRequest, { params }: RouteContext) {
  const { id } = await params;
  if (!isValidID(id)) {
    const { problemResponse } = await import("@/lib/problem-details");
    return problemResponse("Invalid session ID format", 400);
  }
  return proxyRequest({
    path: `/api/charge-sessions/${encodeURIComponent(id)}`,
    method: "DELETE",
  });
}
