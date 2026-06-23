import { proxyGet, proxyRequest } from "@/lib/api-proxy";
import { NextRequest } from "next/server";

export async function PATCH(req: NextRequest) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    const { problemResponse } = await import("@/lib/problem-details");
    return problemResponse("Invalid request body", 400);
  }
  return proxyRequest({ path: "/api/schedule", method: "PATCH", body });
}

export async function GET() {
  return proxyGet({
    path: "/api/schedule",
    detail: "Failed to fetch schedule",
  });
}
