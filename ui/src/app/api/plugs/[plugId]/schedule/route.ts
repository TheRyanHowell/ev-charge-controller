import { proxyGet, proxyRequest } from "@/lib/api-proxy";
import { NextRequest } from "next/server";

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ plugId: string }> },
) {
  const { plugId } = await params;
  return proxyGet({
    path: `/api/plugs/${plugId}/schedule`,
    detail: "Failed to fetch schedule",
  });
}

export async function PATCH(
  req: NextRequest,
  { params }: { params: Promise<{ plugId: string }> },
) {
  const { plugId } = await params;
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    const { problemResponse } = await import("@/lib/problem-details");
    return problemResponse("Invalid request body", 400);
  }
  return proxyRequest({
    path: `/api/plugs/${plugId}/schedule`,
    method: "PATCH",
    body,
  });
}
