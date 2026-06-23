import { proxyGet } from "@/lib/api-proxy";
import { NextRequest } from "next/server";

export async function GET(
  _req: NextRequest,
  { params }: { params: Promise<{ plugId: string }> },
) {
  const { plugId } = await params;
  return proxyGet({
    path: `/api/plugs/${plugId}/provisioning`,
    detail: "Failed to fetch provisioning info",
  });
}
