import { useDragCoordination } from "@/hooks/useDragCoordination";
import { useErrorHandling } from "@/hooks/useErrorHandling";
import { useGaugeStore } from "@/stores/gaugeStore";
import { useEffect, useRef, useState, useCallback } from "react";

import { useChargeActions } from "./useChargeActions";
import { useSessionPolling } from "./useSessionPolling";

export type ChargeSession =
  | { status: "idle" }
  | { status: "pending"; startedAt: number | null }
  | {
      status: "charging" | "conditioning";
      powerDraw: number;
      energyAddedKwh: number | null;
      startedAt: number | null;
      voltage: number | null;
      current: number | null;
    }
  | { status: "error" };

export function useChargeSession(
  selectedVehicle: {
    id: string;
    capacityKwh: number;
    chargerOutputW: number;
    chargingEfficiency: number;
  } | null,
  plugId: string | null,
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
  } | null = null,
) {
  const { errorMessage, onError, clearError } = useErrorHandling();
  const { isDraggingRef, onDragStart, onDragEnd, setCommitting } =
    useDragCoordination();

  const { session, autoStopHandledRef } = useSessionPolling({
    selectedVehicle,
    initialSession,
    isDraggingRef,
  });

  const currentPercent = useGaugeStore((s) => s.currentPercent);
  const targetPercent = useGaugeStore((s) => s.targetPercent);

  const [chargeStartPercent, setChargeStartPercent] = useState<number | null>(
    initialSession?.startPercent ?? null,
  );
  const prevStatusRef = useRef(session.status);
  useEffect(() => {
    if (
      (session.status === "charging" || session.status === "conditioning") &&
      prevStatusRef.current !== "charging" &&
      prevStatusRef.current !== "conditioning" &&
      chargeStartPercent == null
    ) {
      setChargeStartPercent(currentPercent);
    }
    prevStatusRef.current = session.status;
  }, [session.status, currentPercent, chargeStartPercent]);

  const {
    startCharging: startChargingAction,
    stopCharging: stopChargingAction,
    handleTargetChargeUpdate,
  } = useChargeActions({
    vehicle: selectedVehicle,
    plugId,
    currentPercent,
    targetPercent,
    sessionStatus: session.status,
    onError,
    setCommitting,
  });

  const startCharging = useCallback(async () => {
    autoStopHandledRef.current = false;
    if (!selectedVehicle) return;
    try {
      await startChargingAction.mutateAsync({
        vehicleId: selectedVehicle.id,
        startPercent: currentPercent,
        targetPercent,
      });
    } catch {
      // Error handled by mutation's onError callback
    }
  }, [
    selectedVehicle,
    currentPercent,
    targetPercent,
    startChargingAction,
    autoStopHandledRef,
  ]);

  const stopCharging = useCallback(async () => {
    autoStopHandledRef.current = true;
    try {
      await stopChargingAction.mutateAsync();
    } catch {
      // Error handled by mutation's onError callback
    }
  }, [stopChargingAction, autoStopHandledRef]);

  return {
    session,
    chargeStartPercent,
    errorMessage,
    startCharging,
    stopCharging,
    isChargingActionPending: startChargingAction.isPending,
    isStopActionPending: stopChargingAction.isPending,
    handleTargetChargeUpdate,
    onDragStart,
    onDragEnd,
    clearError,
    sessionStartTime:
      session.status === "charging" ||
      session.status === "conditioning" ||
      session.status === "pending"
        ? session.startedAt
        : null,
  };
}
