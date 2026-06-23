import { proxyGet, proxyRequest } from "@/lib/api-proxy";
import { NextRequest } from "next/server";

export async function GET() {
  return proxyGet({
    path: "/api/tariff-settings",
    detail: "Failed to fetch tariff settings",
  });
}

export async function PUT(req: NextRequest) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    const { problemResponse } = await import("@/lib/problem-details");
    return problemResponse("Invalid request body", 400);
  }
  return proxyRequest({ path: "/api/tariff-settings", method: "PUT", body });
}
