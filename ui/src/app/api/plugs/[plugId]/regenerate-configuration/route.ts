import { proxyRequest } from "@/lib/api-proxy";
import { NextRequest } from "next/server";

export async function POST(
  _req: NextRequest,
  { params }: { params: Promise<{ plugId: string }> },
) {
  const { plugId } = await params;
  return proxyRequest({
    path: `/api/plugs/${plugId}/regenerate-configuration`,
    method: "POST",
    body: {},
  });
}
