import { proxyRequest } from "@/lib/api-proxy";
import { NextRequest, NextResponse } from "next/server";

async function proxy(method: string, bodyPromise?: Promise<unknown>) {
  const opts: { path: string; method: string; body?: unknown } = {
    path: "/api/push-subscriptions",
    method,
  };

  if (bodyPromise !== undefined) {
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

export const GET = (): NextResponse => {
  const key = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY ?? "";
  return NextResponse.json({ publicKey: key });
};

export const POST = (req: NextRequest) => proxy("POST", req.json());
export const DELETE = (req: NextRequest) => proxy("DELETE", req.json());
