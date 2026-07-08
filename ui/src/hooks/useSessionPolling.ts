import { apiGet, apiGetSingle, apiPatchNoContent } from "@/lib/api";
import { POLL_INTERVAL_MS } from "@/lib/constants";
import { mapBackendStatus, type SessionStatus } from "@/lib/mappers";

const FRONTEND_STATUSES = new Set([
  "idle",
  "charging",
  "pending",
  "conditioning",
  "holding",
  "error",
] as const);
import { queryKeys } from "@/lib/queryKeys";
import {
  ChargeSessionResponse,
  ChargeSessionResponseSchema,
  HistoryChargeSessionSchema,
} from "@/lib/schemas";
import { useGaugeStore } from "@/stores/gaugeStore";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useMemo, type RefObject } from "react";

import type { ChargeSession } from "./useChargeSession";

function safeMapBackendStatus(status: string): SessionStatus {
  if (FRONTEND_STATUSES.has(status as SessionStatus)) {
    return status as SessionStatus;
  }
  try {
    return mapBackendStatus(status);
  } catch {
    console.warn(`Unknown backend status: ${status}, defaulting to idle`);
    return "idle";
  }
}

interface SessionPollingDeps {
  selectedVehicle: {
    id: string;
    capacityKwh: number;
    chargerOutputW: number;
    chargingEfficiency: number;
  } | null;
  initialSession: {
    status: string;
    powerDraw?: number | null;
    startPercent?: number;
    currentPercent?: number | null;
    targetPercent?: number;
    startedAt?: string | null;
    voltage?: number | null;
    current?: number | null;
    energyAddedKwh?: number | null;
    estimatedResumeTime?: string | null;
  } | null;
  isDraggingRef: RefObject<boolean>;
}

export function useSessionPolling(deps: SessionPollingDeps) {
  const { selectedVehicle, initialSession, isDraggingRef } = deps;
  const autoStopHandledRef = useRef(false);
  const sawActiveSessionRef = useRef(!!initialSession);
  const selectedVehicleRef = useRef(selectedVehicle);
  const lastSyncedDataRef = useRef<string | null>(null);
  const prevApiDataRef = useRef<ChargeSessionResponse | null>(
    initialSession
      ? ({
          status: initialSession.status,
          powerDraw: initialSession.powerDraw ?? null,
          startPercent: initialSession.startPercent ?? null,
          currentPercent: initialSession.currentPercent ?? null,
          targetPercent: initialSession.targetPercent ?? null,
          startedAt: initialSession.startedAt ?? null,
          voltage: initialSession.voltage ?? null,
          current: initialSession.current ?? null,
          energyAddedKwh: initialSession.energyAddedKwh ?? null,
          estimatedResumeTime: initialSession.estimatedResumeTime ?? null,
        } as ChargeSessionResponse)
      : null,
  );
  const queryClient = useQueryClient();

  useEffect(() => {
    selectedVehicleRef.current = selectedVehicle;
  }, [selectedVehicle]);

  // Scope query key to vehicle ID so each vehicle has its own cached session.
  // When the vehicle changes, React Query creates a fresh query entry,
  // returning undefined until the fetch completes → session becomes "idle".
  const queryKey = selectedVehicle
    ? queryKeys.chargeSession.byVehicle(selectedVehicle.id)
    : queryKeys.chargeSession.none;

  // Reset gauge synced data when vehicle changes so the gauge sync fires
  // for the new vehicle's session data. Also clear stale query cache.
  // Reset auto-stop refs so a null result for the new (idle) vehicle does not
  // incorrectly trigger auto-stop logic that belongs to the previous vehicle's session.
  const prevVehicleIdRef = useRef<string | null>(selectedVehicle?.id ?? null);
  useEffect(() => {
    const vehicleId = selectedVehicle?.id ?? null;
    const oldVehicleId = prevVehicleIdRef.current;
    if (oldVehicleId !== vehicleId) {
      prevVehicleIdRef.current = vehicleId;
      lastSyncedDataRef.current = null;
      sawActiveSessionRef.current = false;
      prevApiDataRef.current = null;
      autoStopHandledRef.current = false;
      // Clear stale query data from the old global key
      queryClient.setQueryData(queryKeys.chargeSession.all, undefined);
      // Remove old vehicle's cached query so the new vehicle fetches fresh
      if (oldVehicleId) {
        queryClient.removeQueries({
          queryKey: queryKeys.chargeSession.byVehicle(oldVehicleId),
        });
      }
    }
  }, [selectedVehicle, queryClient]);

  // Pre-compute initial session data from SSR (mapped to ChargeSessionResponse shape).
  // This is used as initialData on useQuery to avoid the need for useEffect + setQueryData.
  const initialSessionData: ChargeSessionResponse | undefined = initialSession
    ? ({
        status: initialSession.status,
        powerDraw: initialSession.powerDraw ?? null,
        startPercent: initialSession.startPercent ?? null,
        currentPercent: initialSession.currentPercent ?? null,
        targetPercent: initialSession.targetPercent ?? null,
        startedAt: initialSession.startedAt ?? null,
        voltage: initialSession.voltage ?? null,
        current: initialSession.current ?? null,
        energyAddedKwh: initialSession.energyAddedKwh ?? null,
        estimatedResumeTime: initialSession.estimatedResumeTime ?? null,
      } as ChargeSessionResponse)
    : undefined;

  const queryDataRaw = useQuery<ChargeSessionResponse | null>({
    queryKey,
    queryFn: async ({ signal }) => {
      const vehicleId = selectedVehicleRef.current?.id;
      if (!vehicleId) return null;
      return apiGetSingle(
        `/api/charge-sessions?vehicleId=${encodeURIComponent(vehicleId)}`,
        ChargeSessionResponseSchema,
        { signal },
      );
    },
    enabled: !!selectedVehicle,
    refetchInterval: POLL_INTERVAL_MS,
    refetchIntervalInBackground: true,
    refetchOnMount: true,
    staleTime: 0,
    initialData: initialSessionData,
    // Data is immediately stale so it refetches in background instantly
    initialDataUpdatedAt: 0,
  });

  const queryData = queryDataRaw.data;

  // Derive session from query data (no setState-in-effect)
  const session: ChargeSession = useMemo(() => {
    if (queryData === undefined || queryData === null) {
      return { status: "idle" };
    }
    const status = safeMapBackendStatus(queryData.status);
    const chargerOutputW = selectedVehicle?.chargerOutputW ?? 0;
    const startedAt = queryData.startedAt
      ? new Date(queryData.startedAt).getTime()
      : null;
    if (
      status === "charging" ||
      status === "conditioning" ||
      status === "holding"
    ) {
      return {
        status,
        powerDraw: queryData.powerDraw ?? chargerOutputW,
        energyAddedKwh: queryData.energyAddedKwh ?? null,
        startedAt,
        voltage: queryData.voltage ?? null,
        current: queryData.current ?? null,
        estimatedResumeTime: queryData.estimatedResumeTime ?? null,
      };
    }
    if (status === "pending") {
      return { status, startedAt };
    }
    return { status };
  }, [queryData, selectedVehicle]);

  // Gauge store sync (side effect only, no setState)
  const setCurrentPercent = useGaugeStore((s) => s.setCurrentPercent);
  const setTargetPercent = useGaugeStore((s) => s.setTargetPercent);
  useEffect(() => {
    if (!queryData || isDraggingRef.current) return;
    const cp = queryData.currentPercent ?? queryData.startPercent ?? 0;
    const tp = queryData.targetPercent;
    const dataStr = JSON.stringify({ cp, tp });
    if (dataStr !== lastSyncedDataRef.current) {
      lastSyncedDataRef.current = dataStr;
      setCurrentPercent(cp);
      if (tp != null) {
        setTargetPercent(tp);
      }
    }
  }, [
    queryData,
    isDraggingRef,
    lastSyncedDataRef,
    setCurrentPercent,
    setTargetPercent,
  ]);

  // Auto-stop detection: when queryData transitions from active to null
  const setPercents = useGaugeStore((s) => s.setPercents);
  useEffect(() => {
    if (queryData === undefined) return;
    if (queryData !== null) {
      sawActiveSessionRef.current = true;
      prevApiDataRef.current = queryData;
      return;
    }
    const prevApiData = prevApiDataRef.current;
    const shouldCheckAutoStop =
      prevApiData !== null &&
      !autoStopHandledRef.current &&
      sawActiveSessionRef.current;
    prevApiDataRef.current = null;
    if (!shouldCheckAutoStop) return;
    const sv = selectedVehicleRef.current;
    if (!sv) return;
    const ctrl = new AbortController();
    (async () => {
      try {
        const sessions = await apiGet(
          `/api/history?vehicleId=${sv.id}&limit=1`,
          HistoryChargeSessionSchema,
          { signal: ctrl.signal },
        );
        const cs = sessions[0];
        if (cs && cs.status === "completed") {
          const cp = cs.endPercent ?? useGaugeStore.getState().currentPercent;
          const tp = Math.min(
            100,
            Math.max(
              0,
              cp > (cs.targetPercent ?? 0)
                ? Math.round(cp + 10)
                : (cs.targetPercent ?? 0),
            ),
          );
          setPercents(cp, tp);
          // Persist to vehicle API so idle gauge sync doesn't overwrite with stale data
          await apiPatchNoContent(`/api/vehicles/${sv.id}`, {
            currentPercent: cp,
            targetPercent: tp,
          });
          queryClient.invalidateQueries({ queryKey: queryKeys.vehicles.all });
          autoStopHandledRef.current = true;
        }
      } catch (err) {
        if (err instanceof DOMException && err.name === "AbortError") return;
        console.error("Failed to fetch history for auto-stop:", err);
      }
    })();
    return () => ctrl.abort();
  }, [queryData, setPercents, queryClient]);

  return {
    session,
    autoStopHandledRef,
  };
}
