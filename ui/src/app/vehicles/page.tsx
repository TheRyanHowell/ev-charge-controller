import type { Vehicle, VehicleModel } from "@/lib/schemas";

import { refreshAccessTokenPair } from "@/lib/auth-refresh";
import { VehicleSchema, VehicleModelSchema } from "@/lib/schemas";
import { serverFetch } from "@/lib/server-fetch";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import VehiclesClient from "./VehiclesClient";

function serverRenderTimeMs(): number {
  return Date.now();
}

async function fetchVehicles(
  accessToken?: string,
): Promise<{ data: Vehicle[]; error: boolean }> {
  try {
    const res = await serverFetch("/api/vehicles", { accessToken });
    if (res.ok) {
      const json = await res.json();
      return { data: VehicleSchema.array().parse(json), error: false };
    }
    return { data: [], error: true };
  } catch {
    return { data: [], error: true };
  }
}

async function fetchModels(accessToken?: string): Promise<VehicleModel[]> {
  try {
    const res = await serverFetch("/api/vehicle-models", { accessToken });
    if (res.ok) {
      const json = await res.json();
      return VehicleModelSchema.array().parse(json);
    }
  } catch {
    // Network error is non-fatal
  }
  return [];
}

export default async function VehiclesPage() {
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
  const [vehiclesResult, models] = await Promise.all([
    fetchVehicles(accessToken),
    fetchModels(accessToken),
  ]);

  return (
    <VehiclesClient
      initialVehicles={vehiclesResult.data}
      initialError={vehiclesResult.error}
      initialModels={models}
      renderTimeMs={renderTimeMs}
    />
  );
}
