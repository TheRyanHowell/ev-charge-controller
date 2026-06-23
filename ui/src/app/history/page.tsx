import type { HistoryChargeSession, HistoryVehicle } from "@/lib/schemas";

import { refreshAccessTokenPair } from "@/lib/auth-refresh";
import {
  HistoryVehicleSchema,
  HistoryChargeSessionSchema,
} from "@/lib/schemas";
import { serverFetch } from "@/lib/server-fetch";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import HistoryClient from "./HistoryClient";

async function fetchHistoryVehicles(
  accessToken?: string,
): Promise<HistoryVehicle[]> {
  try {
    const res = await serverFetch("/api/vehicles", { accessToken });
    if (res.ok) {
      const json = await res.json();
      return HistoryVehicleSchema.array().parse(json);
    }
  } catch {
    // Network error during vehicle fetch is non-fatal
  }
  return [];
}

async function fetchHistorySessions(
  date: string,
  accessToken?: string,
): Promise<HistoryChargeSession[]> {
  try {
    const res = await serverFetch(
      `/api/history?date=${date}&limit=50&offset=0`,
      { accessToken },
    );
    if (res.status === 204) {
      return [];
    }
    if (res.ok) {
      const json = await res.json();
      return HistoryChargeSessionSchema.array().parse(json);
    }
  } catch {
    // Network error during session fetch is non-fatal
  }
  return [];
}

async function fetchLatestSessionDate(
  accessToken?: string,
): Promise<string | null> {
  try {
    const res = await serverFetch("/api/history?limit=1&offset=0", {
      accessToken,
    });
    if (res.ok) {
      const json = await res.json();
      const sessions = HistoryChargeSessionSchema.array().parse(json);
      const first = sessions[0];
      if (first) {
        return first.createdAt.split("T")[0] ?? null;
      }
    }
  } catch {
    // Non-fatal
  }
  return null;
}

export default async function HistoryPage() {
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

  const today = new Date().toISOString().split("T")[0] ?? "";

  const [vehicles, todaySessions] = await Promise.all([
    fetchHistoryVehicles(accessToken),
    fetchHistorySessions(today, accessToken),
  ]);

  if (todaySessions.length > 0) {
    return (
      <HistoryClient
        initialVehicles={vehicles}
        initialSessions={todaySessions}
        initialDate={today}
      />
    );
  }

  // No sessions today - default to the most recent session's date.
  const latestDate = await fetchLatestSessionDate(accessToken);
  if (latestDate && latestDate !== today) {
    const latestSessions = await fetchHistorySessions(latestDate, accessToken);
    return (
      <HistoryClient
        initialVehicles={vehicles}
        initialSessions={latestSessions}
        initialDate={latestDate}
      />
    );
  }

  return (
    <HistoryClient
      initialVehicles={vehicles}
      initialSessions={[]}
      initialDate={today}
    />
  );
}
