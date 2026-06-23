import type { Vehicle, VehicleStats } from "@/lib/schemas";

import { refreshAccessTokenPair } from "@/lib/auth-refresh";
import { VehicleSchema, VehicleStatsSchema } from "@/lib/schemas";
import { serverFetch } from "@/lib/server-fetch";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import VehicleDetailClient from "./VehicleDetailClient";

function serverRenderTimeMs(): number {
  return Date.now();
}

async function fetchVehicle(
  accessToken?: string,
  vehicleId?: string,
): Promise<Vehicle | null> {
  if (!vehicleId) return null;
  try {
    const res = await serverFetch(`/api/vehicles/${vehicleId}`, {
      accessToken,
    });
    if (res.ok) {
      const json = await res.json();
      return VehicleSchema.parse(json);
    }
  } catch {
    // Network error
  }
  return null;
}

async function fetchStats(
  accessToken?: string,
  vehicleId?: string,
  range = "week",
): Promise<VehicleStats | null> {
  if (!vehicleId) return null;
  try {
    const res = await serverFetch(
      `/api/vehicles/${vehicleId}/stats?range=${range}`,
      { accessToken },
    );
    if (res.ok) {
      const json = await res.json();
      return VehicleStatsSchema.parse(json);
    }
  } catch {
    // Network error
  }
  return null;
}

export default async function VehicleDetailPage({
  params,
}: {
  params: Promise<{ vehicleId: string }>;
}) {
  const { vehicleId } = await params;
  const cookieStore = await cookies();
  let accessToken = cookieStore.get("access_token")?.value;
  const refreshToken = cookieStore.get("refresh_token")?.value;

  if (!accessToken && !refreshToken) {
    redirect("/login");
  }

  if (!accessToken && refreshToken) {
    const tokens = await refreshAccessTokenPair();
    if (!tokens) {
      redirect("/login");
    }
    accessToken = tokens.accessToken;
  }

  const renderTimeMs = serverRenderTimeMs();
  const [vehicle, stats] = await Promise.all([
    fetchVehicle(accessToken, vehicleId),
    fetchStats(accessToken, vehicleId, "week"),
  ]);

  if (!vehicle) {
    redirect("/vehicles");
  }

  return (
    <VehicleDetailClient
      vehicleId={vehicleId}
      initialVehicle={vehicle}
      initialStats={stats}
      renderTimeMs={renderTimeMs}
    />
  );
}
