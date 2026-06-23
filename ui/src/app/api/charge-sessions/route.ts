import { proxyRequest } from "@/lib/api-proxy";
import { problemResponse } from "@/lib/problem-details";
import { isValidID } from "@/lib/validation";
import { NextRequest, NextResponse } from "next/server";

async function proxy(
  method: string,
  req: NextRequest,
  bodyPromise?: Promise<unknown>,
) {
  const vehicleId = req.nextUrl.searchParams.get("vehicleId");
  const path = vehicleId
    ? `/api/charge-sessions?vehicleId=${encodeURIComponent(vehicleId)}`
    : "/api/charge-sessions";

  const opts: { path: string; method: string; body?: unknown } = {
    path,
    method,
  };

  if (bodyPromise !== undefined && method !== "GET") {
    try {
      opts.body = await bodyPromise;
    } catch {
      const { problemResponse } = await import("@/lib/problem-details");
      return problemResponse("Invalid request body", 400);
    }
  }

  const res = await proxyRequest(opts);
  if (res.status === 204) {
    return new NextResponse(null, { status: 204 });
  }
  return res;
}

export const GET = (req: NextRequest) => proxy("GET", req);
export const POST = (req: NextRequest) => proxy("POST", req, req.json());
export const DELETE = (req: NextRequest) => {
  const sessionId = req.nextUrl.searchParams.get("id");
  if (!sessionId || !isValidID(sessionId)) {
    return problemResponse("Invalid session ID format", 400);
  }
  return proxyRequest({
    path: `/api/charge-sessions/${sessionId}`,
    method: "DELETE",
  });
};
export const PATCH = (req: NextRequest) => proxy("PATCH", req, req.json());
