import type { Vehicle } from "@/lib/schemas";

import { calculateETA } from "@/utils/eta";
import { useEffect, useState } from "react";

export type ChargeStatus =
  | "idle"
  | "charging"
  | "pending"
  | "conditioning"
  | "holding"
  | "error";

interface UseElapsedEtaParams {
  status: ChargeStatus;
  sessionStartTime: number | null;
  currentPercent: number;
  targetPercent: number;
  vehicle: Vehicle | null;
  /** Server render time, used as the idle base so SSR and client agree. */
  renderTimeMs?: number;
  /** Battery-side energy added this session; enables live ETA calibration. */
  energyAddedKwh?: number | null;
  /** Session start percent; enables live ETA calibration. */
  startPercent?: number | null;
}

interface ElapsedEta {
  /** Milliseconds elapsed since the session started (0 when idle). */
  elapsed: number;
  /** Total charge estimate in minutes (elapsed + remaining while charging), or null when not estimable. */
  totalTimeMin: number | null;
  /** Reference clock time the target-completion display counts from. */
  baseTime: number;
  /** Whole seconds remaining to target, or null when not estimable. */
  remainingSec: number | null;
}

/**
 * useElapsedEta owns the charge timing model for StatsPanel: it ticks the
 * elapsed timer every second and derives Time Remaining from the LIVE
 * ETA (current SOC → target) on every render. Remaining is deliberately NOT
 * an anchored countdown: a countdown clamps to 0 when charging runs slower
 * than the original quote, silently lying to the user. The live estimate
 * re-derives as the polled SOC advances, so duration, remaining, and target
 * completion always describe the charge as it is actually progressing.
 */
export function useElapsedEta({
  status,
  sessionStartTime,
  currentPercent,
  targetPercent,
  vehicle,
  renderTimeMs,
  energyAddedKwh,
  startPercent,
}: UseElapsedEtaParams): ElapsedEta {
  // Live estimate from the current SOC to target, recomputed every render so
  // it tracks polled progress. Session telemetry (energy added vs SOC gained)
  // calibrates the estimate to the battery's observed behavior when available.
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
        energyAddedKwh: energyAddedKwh ?? null,
        startPercent: startPercent ?? null,
      })
    : null;
  const liveEtaMin =
    computedEta != null && computedEta > 0 ? computedEta : null;

  const [elapsed, setElapsed] = useState(0);
  // Idle uses renderTimeMs (server render time) so SSR matches client;
  // charging uses sessionStartTime. The tick effect below runs immediately on
  // mount and advances this to the live clock, so the initial value stays pure
  // (no Date.now() during render → no hydration mismatch).
  const [baseTime, setBaseTime] = useState(
    sessionStartTime ?? renderTimeMs ?? 0,
  );

  // Tick the elapsed timer (and idle base clock) once per second.
  useEffect(() => {
    const tick = () => {
      if (sessionStartTime && status !== "idle") {
        setElapsed(Date.now() - sessionStartTime);
        setBaseTime(sessionStartTime);
      } else {
        setElapsed(0);
        // When idle, advance baseTime so target completion ticks up.
        setBaseTime(Date.now());
      }
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [sessionStartTime, status]);

  const isChargingSession = sessionStartTime != null && status !== "idle";

  // Remaining is the live estimate; once the target is reached but the session
  // is still wrapping up (conditioning/holding tail), report 0 rather than "-".
  const remainingSec =
    liveEtaMin != null
      ? Math.round(liveEtaMin * 60)
      : isChargingSession && vehicle != null && currentPercent >= targetPercent
        ? 0
        : null;

  // Target completion = baseTime + totalTimeMin. While charging that must be
  // "now + remaining", i.e. sessionStart + elapsed + remaining.
  const totalTimeMin =
    remainingSec == null
      ? null
      : isChargingSession
        ? elapsed / 60000 + remainingSec / 60
        : remainingSec / 60;

  return { elapsed, totalTimeMin, baseTime, remainingSec };
}
