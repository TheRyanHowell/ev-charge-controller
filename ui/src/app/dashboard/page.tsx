import type {
  CarbonIntensity,
  Vehicle,
  Plug,
  InitialChargeSession,
  Schedule,
} from "@/lib/schemas";
import type { PowerReading, SOCSnapshot } from "@/types/chart";

import {
  InitialChargeSessionSchema,
  VehicleSchema,
  PlugSchema,
  CarbonIntensitySchema,
  PowerReadingSchema,
  ScheduleSchema,
  SocSnapshotSchema,
} from "@/lib/schemas";
import { serverFetch } from "@/lib/server-fetch";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import Dashboard from "../Dashboard";

type FetchResult<T> = { authorized: true; data: T } | { authorized: false };

export const metadata = {
  title: "Dashboard - EV Charge Controller",
};

function serverRenderTimeMs(): number {
  return Date.now();
}

async function fetchVehicles(
  accessToken?: string,
): Promise<FetchResult<Vehicle[]>> {
  try {
    const res = await serverFetch("/api/vehicles", { accessToken });
    if (res.status === 401) return { authorized: false };
    if (res.ok) {
      const json = await res.json();
      return { authorized: true, data: VehicleSchema.array().parse(json) };
    }
  } catch (err) {
    if (err instanceof Error && err.message === "Authentication required") {
      return { authorized: false };
    }
  }
  return { authorized: true, data: [] };
}

async function fetchPlugs(accessToken?: string): Promise<FetchResult<Plug[]>> {
  try {
    const res = await serverFetch("/api/plugs", { accessToken });
    if (res.status === 401) return { authorized: false };
    if (res.ok) {
      const json = await res.json();
      return { authorized: true, data: PlugSchema.array().parse(json) };
    }
  } catch (err) {
    if (err instanceof Error && err.message === "Authentication required") {
      return { authorized: false };
    }
  }
  return { authorized: true, data: [] };
}

async function fetchActiveSession(
  accessToken?: string,
  vehicleId?: string | null,
): Promise<InitialChargeSession | null> {
  try {
    const url = vehicleId
      ? `/api/charge-sessions?vehicleId=${encodeURIComponent(vehicleId)}`
      : "/api/charge-sessions";
    const res = await serverFetch(url, { accessToken });
    if (res.status === 204) return null;
    if (res.ok) {
      const json = await res.json();
      return InitialChargeSessionSchema.parse({
        ...json,
        renderTimeMs: Date.now(),
      });
    }
  } catch {
    // non-fatal
  }
  return null;
}

async function fetchCarbonIntensity(
  accessToken?: string,
): Promise<CarbonIntensity | null> {
  try {
    const res = await serverFetch("/api/carbon-intensity", { accessToken });
    if (res.ok) {
      const json = await res.json();
      return CarbonIntensitySchema.parse(json);
    }
  } catch {
    // non-fatal
  }
  return null;
}

async function fetchInitialPowerReadings(
  accessToken?: string,
): Promise<PowerReading[]> {
  try {
    const res = await serverFetch("/api/power-readings", { accessToken });
    if (res.ok) {
      const json = await res.json();
      return PowerReadingSchema.array().parse(json);
    }
  } catch {
    // non-fatal
  }
  return [];
}

async function fetchInitialSocSnapshots(
  accessToken?: string,
): Promise<SOCSnapshot[]> {
  try {
    const res = await serverFetch("/api/soc-snapshots", { accessToken });
    if (res.ok) {
      const json = await res.json();
      return SocSnapshotSchema.array().parse(json);
    }
  } catch {
    // non-fatal
  }
  return [];
}

async function fetchSchedule(
  accessToken?: string,
  plugId?: string | null,
): Promise<Schedule | null> {
  if (!plugId) return null;
  try {
    const res = await serverFetch(`/api/plugs/${plugId}/schedule`, {
      accessToken,
    });
    if (res.ok) {
      const json = await res.json();
      return ScheduleSchema.parse(json);
    }
  } catch {
    // non-fatal
  }
  return null;
}

export default async function DashboardPage() {
  const cookieStore = await cookies();
  const accessToken = cookieStore.get("access_token")?.value;

  if (!accessToken) {
    redirect("/login");
  }

  // Fetch plugs and vehicles in parallel first (needed to determine selected plug/vehicle).
  const [vehiclesResult, plugsResult] = await Promise.all([
    fetchVehicles(accessToken),
    fetchPlugs(accessToken),
  ]);

  if (!plugsResult.authorized || !vehiclesResult.authorized) {
    redirect("/login?reason=session-expired");
  }

  const plugs = plugsResult.data;
  const vehicles = vehiclesResult.data;

  // Restore selected vehicle from cookie (persisted across page reloads).
  const selectedVehicleIdCookie = cookieStore.get("selected_vehicle_id")?.value;
  const vehicleIds = new Set(vehicles.map((v) => v.id));
  const initialSelectedVehicleId =
    selectedVehicleIdCookie && vehicleIds.has(selectedVehicleIdCookie)
      ? selectedVehicleIdCookie
      : (vehicles[0]?.id ?? null);

  // Derive the charging plug for the selected vehicle (for session + schedule SSR).
  const chargingPlug = plugs.find(
    (p) => p.type === "charging" && p.vehicleId === initialSelectedVehicleId,
  );

  // Fetch remaining data for the selected vehicle and its charging plug.
  const [session, carbonIntensity, powerReadings, socSnapshots, schedule] =
    await Promise.all([
      fetchActiveSession(accessToken, initialSelectedVehicleId),
      fetchCarbonIntensity(accessToken),
      fetchInitialPowerReadings(accessToken),
      fetchInitialSocSnapshots(accessToken),
      fetchSchedule(accessToken, chargingPlug?.id ?? null),
    ]);

  return (
    <Dashboard
      initialVehicles={vehicles}
      initialPlugs={plugs}
      initialSession={session}
      initialCarbonIntensity={carbonIntensity}
      initialPowerReadings={powerReadings}
      initialSocSnapshots={socSnapshots}
      initialSchedule={schedule}
      initialSelectedVehicleId={initialSelectedVehicleId}
      renderTimeMs={serverRenderTimeMs()}
    />
  );
}
