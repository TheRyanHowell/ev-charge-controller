import type { Vehicle } from "@/lib/schemas";

import { calculateETA } from "@/utils/eta";
import { useEffect, useRef, useState } from "react";

export type ChargeStatus =
  | "idle"
  | "charging"
  | "pending"
  | "conditioning"
  | "error";

interface UseElapsedEtaParams {
  status: ChargeStatus;
  sessionStartTime: number | null;
  currentPercent: number;
  targetPercent: number;
  vehicle: Vehicle | null;
  /** Server render time, used as the idle base so SSR and client agree. */
  renderTimeMs?: number;
}

interface ElapsedEta {
  /** Milliseconds elapsed since the session started (0 when idle). */
  elapsed: number;
  /** Anchored total charge estimate in minutes, or null when not estimable. */
  totalTimeMin: number | null;
  /** Reference clock time the target-completion display counts from. */
  baseTime: number;
  /** Whole seconds remaining to target, or null when not estimable. */
  remainingSec: number | null;
}

/**
 * useElapsedEta owns the charge timing model for StatsPanel: it ticks the
 * elapsed timer every second and anchors the total-time estimate so that
 * `duration + remaining` stays equal to the originally-quoted ETA across the
 * session, target changes, and idle gauge dragging. Extracted from StatsPanel
 * so that component is a pure presentational grid.
 */
export function useElapsedEta({
  status,
  sessionStartTime,
  currentPercent,
  targetPercent,
  vehicle,
  renderTimeMs,
}: UseElapsedEtaParams): ElapsedEta {
  // Computed synchronously so useState can seed from it on first render.
  const computedEta = vehicle
    ? calculateETA({
        currentPercent,
        targetPercent,
        capacityKwh: vehicle.capacityKwh,
        chargerOutputW: vehicle.chargerOutputW,
        chargingEfficiency: vehicle.chargingEfficiency,
        time0to80Min: vehicle.time0to80Min ?? null,
        time0to100Min: vehicle.time0to100Min ?? null,
        time20to80Min: vehicle.time20to80Min ?? null,
        time20to100Min: vehicle.time20to100Min ?? null,
      })
    : null;

  const [elapsed, setElapsed] = useState(0);
  // Seed from computedEta so Time Remaining / Target Completion render at FCP.
  const [totalTimeMin, setTotalTimeMin] = useState<number | null>(() =>
    computedEta != null && computedEta > 0 ? computedEta : null,
  );
  // Idle uses renderTimeMs (server render time) so SSR matches client;
  // charging uses sessionStartTime. The tick effect below runs immediately on
  // mount and advances this to the live clock, so the initial value stays pure
  // (no Date.now() during render → no hydration mismatch).
  const [baseTime, setBaseTime] = useState(
    sessionStartTime ?? renderTimeMs ?? 0,
  );
  const totalEstimatedMinRef = useRef<number | null>(null);
  const prevEtaRef = useRef<number | null>(null);
  const prevSessionRef = useRef<number | null>(null);
  const prevTargetRef = useRef<number>(targetPercent);
  const elapsedRef = useRef(0);

  // Tick the elapsed timer (and idle base clock) once per second.
  useEffect(() => {
    const tick = () => {
      if (sessionStartTime && status !== "idle") {
        const e = Date.now() - sessionStartTime;
        setElapsed(e);
        elapsedRef.current = e;
        setBaseTime(sessionStartTime);
      } else {
        setElapsed(0);
        elapsedRef.current = 0;
        // When idle, advance baseTime so target completion ticks up.
        setBaseTime(Date.now());
      }
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [sessionStartTime, status]);

  // Anchor the total-time estimate across session start, target changes, idle drag.
  useEffect(() => {
    const sessionChanged = sessionStartTime !== prevSessionRef.current;
    const targetChanged = targetPercent !== prevTargetRef.current;
    const isCharging = sessionStartTime != null;

    if (computedEta != null && computedEta > 0) {
      if (sessionChanged) {
        // Session started: anchor to the initial ETA so
        // duration + remaining = computedEta * 60s always.
        totalEstimatedMinRef.current = computedEta;
        setTotalTimeMin(computedEta);
      } else if (isCharging && targetChanged) {
        // Target changed mid-charge: add elapsed so remaining shows the full
        // ETA from current SOC to the new target at this moment.
        const total = computedEta + elapsedRef.current / 60000;
        totalEstimatedMinRef.current = total;
        setTotalTimeMin(total);
      } else if (!isCharging) {
        // Idle: live-update estimate as the user drags gauge handles.
        totalEstimatedMinRef.current = computedEta;
        setTotalTimeMin(computedEta);
      }
      // Active charge, no target change: leave totalTimeMin unchanged so
      // duration + remaining stays constant.
      prevEtaRef.current = computedEta;
    } else if (!vehicle || currentPercent >= targetPercent) {
      totalEstimatedMinRef.current = null;
      setTotalTimeMin(null);
    }
    prevSessionRef.current = sessionStartTime;
    prevTargetRef.current = targetPercent;
  }, [
    computedEta,
    sessionStartTime,
    currentPercent,
    status,
    vehicle,
    targetPercent,
  ]);

  // Reset anchoring refs when returning to idle.
  useEffect(() => {
    if (status === "idle") {
      prevEtaRef.current = null;
      prevSessionRef.current = null;
    }
  }, [status]);

  // Compute remaining in whole seconds so duration + remaining = totalTimeSec
  // exactly (both displays floor the same integer elapsedSec).
  const totalTimeSec =
    totalTimeMin != null ? Math.round(totalTimeMin * 60) : null;
  const elapsedSec = Math.floor(elapsed / 1000);
  const remainingSec =
    totalTimeSec != null ? Math.max(0, totalTimeSec - elapsedSec) : null;

  return { elapsed, totalTimeMin, baseTime, remainingSec };
}
