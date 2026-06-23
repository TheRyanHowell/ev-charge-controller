import { apiPatchNoContent, apiPost } from "@/lib/api";
import { TARGET_UPDATE_DEBOUNCE_MS } from "@/lib/constants";
import { mapBackendStatus } from "@/lib/mappers";
import { queryKeys } from "@/lib/queryKeys";
import {
  ChargeSessionResponse,
  ChargeSessionResponseSchema,
} from "@/lib/schemas";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useRef, useEffect } from "react";

export interface ChargeActionsDeps {
  vehicle: {
    id: string;
    capacityKwh: number;
    chargerOutputW: number;
    chargingEfficiency: number;
  } | null;
  plugId: string | null;
  currentPercent: number;
  targetPercent: number;
  sessionStatus: "idle" | "charging" | "pending" | "conditioning" | "error";
  onError: (msg: string) => void;
  onTargetUpdateError?: (msg: string) => void;
  // Suppresses inbound gauge syncs while a target commit is in flight.
  setCommitting?: (committing: boolean) => void;
}

interface StartChargingVars {
  vehicleId: string;
  startPercent: number;
  targetPercent: number;
}

interface UpdateTargetVars {
  targetPercent: number;
}

interface ChargeAction<TVars> {
  mutateAsync: (vars: TVars) => Promise<unknown>;
  isPending: boolean;
}

export function useChargeActions(deps: ChargeActionsDeps) {
  const { vehicle, plugId, sessionStatus, onError } = deps;

  const chargerOutputW = vehicle?.chargerOutputW ?? 0;
  const queryClient = useQueryClient();

  // Scope query key to vehicle ID so mutations target the correct cached session
  const queryKey = vehicle
    ? queryKeys.chargeSession.byVehicle(vehicle.id)
    : queryKeys.chargeSession.none;

  const updateTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const prevChargingPowerRef = useRef<number>(chargerOutputW);
  const sessionStatusRef = useRef(sessionStatus);

  useEffect(() => {
    sessionStatusRef.current = sessionStatus;
  }, [sessionStatus]);

  useEffect(() => {
    return () => {
      if (updateTimerRef.current) {
        clearTimeout(updateTimerRef.current);
        updateTimerRef.current = null;
      }
    };
  }, []);

  const startMutation = useMutation({
    mutationFn: async (
      vars: StartChargingVars,
    ): Promise<ChargeSessionResponse> =>
      apiPost("/api/charge-sessions", ChargeSessionResponseSchema, {
        vehicleId: vars.vehicleId,
        startPercent: vars.startPercent,
        targetPercent: vars.targetPercent,
        ...(plugId && { plugId }),
      }),
    onMutate: (vars) => {
      queryClient.setQueryData(queryKey, {
        status: "pending",
        startPercent: vars.startPercent,
        currentPercent: vars.startPercent,
        targetPercent: vars.targetPercent,
      });
    },
    onSuccess: (data) => {
      const status = mapBackendStatus(data.status);
      if (status === "charging") {
        prevChargingPowerRef.current = data.powerDraw ?? chargerOutputW;
      }
      queryClient.setQueryData(queryKey, data);
    },
    onError: (error) => {
      const msg = error instanceof Error ? error.message : "Unknown error";
      onError(msg);
      queryClient.setQueryData(queryKey, { status: "idle" });
    },
    onSettled: () => {
      void queryClient.invalidateQueries({
        queryKey,
      });
    },
  });

  const stopMutation = useMutation({
    mutationFn: () => {
      const vehicleId = vehicle?.id;
      if (!vehicleId) throw new Error("No vehicle selected");
      return apiPatchNoContent(
        `/api/charge-sessions?vehicleId=${encodeURIComponent(vehicleId)}`,
        {
          status: "stopped",
        },
      );
    },
    onMutate: async () => {
      await queryClient.cancelQueries({
        queryKey,
      });
      const prevData = queryClient.getQueryData(queryKey);
      queryClient.setQueryData(queryKey, null);
      return { prevData };
    },
    onError: (_error, _vars, context) => {
      onError((_error as Error).message);
      if (context?.prevData !== undefined) {
        queryClient.setQueryData(queryKey, context.prevData);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({
        queryKey,
      });
    },
  });

  const updateTargetMutation = useMutation({
    mutationFn: async (vars: UpdateTargetVars) => {
      const vehicleId = vehicle?.id;
      if (!vehicleId) throw new Error("No vehicle selected");
      return apiPatchNoContent(
        `/api/charge-sessions?vehicleId=${encodeURIComponent(vehicleId)}`,
        {
          targetPercent: vars.targetPercent,
        },
      );
    },
    onMutate: async (vars) => {
      // Cancel in-flight refetches so a stale poll can't overwrite the cache,
      // then optimistically reflect the new target in the session cache.
      await queryClient.cancelQueries({ queryKey });
      const prevData = queryClient.getQueryData<ChargeSessionResponse | null>(
        queryKey,
      );
      queryClient.setQueryData<ChargeSessionResponse | null>(queryKey, (old) =>
        old ? { ...old, targetPercent: vars.targetPercent } : old,
      );
      return { prevData };
    },
    onError: (error, _vars, context) => {
      if (context?.prevData !== undefined) {
        queryClient.setQueryData(queryKey, context.prevData);
      }
      const msg =
        error instanceof Error
          ? error.message
          : "Failed to update target percent";
      console.error("Target percent update failed:", msg);
      if (deps.onTargetUpdateError) {
        deps.onTargetUpdateError(msg);
      }
    },
  });

  const setCommitting = deps.setCommitting;
  const handleTargetChargeUpdate = useCallback(
    (_current: number, target: number) => {
      if (updateTimerRef.current) clearTimeout(updateTimerRef.current);
      // Hold the sync guard immediately (covers the debounce window before the
      // request even fires) and release it once the write settles.
      setCommitting?.(true);
      updateTimerRef.current = setTimeout(async () => {
        const status = sessionStatusRef.current;
        if (status !== "charging" && status !== "conditioning") {
          setCommitting?.(false);
          return;
        }
        try {
          await updateTargetMutation.mutateAsync({ targetPercent: target });
        } catch {
          // Error already handled in onError callback
        } finally {
          setCommitting?.(false);
        }
      }, TARGET_UPDATE_DEBOUNCE_MS);
    },
    [updateTargetMutation, setCommitting],
  );

  return {
    startCharging: {
      mutateAsync: startMutation.mutateAsync,
      isPending: startMutation.isPending,
    } as ChargeAction<StartChargingVars>,
    stopCharging: {
      mutateAsync: stopMutation.mutateAsync,
      isPending: stopMutation.isPending,
    } as ChargeAction<void>,
    handleTargetChargeUpdate,
  };
}
