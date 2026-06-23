import type { Vehicle } from "@/lib/schemas";

import { apiGet, apiPatchRaw } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import { VehicleSchema } from "@/lib/schemas";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

import { useSettingsModal } from "./useSettingsModal";
import { useTempError } from "./useTempError";

async function fetchVehiclesList(): Promise<Vehicle[]> {
  return await apiGet("/api/vehicles", VehicleSchema);
}

export function useVehicle({
  initialVehicles,
  initialDataUpdatedAt,
}: {
  initialVehicles?: Vehicle[];
  initialDataUpdatedAt?: number;
} = {}) {
  const queryClient = useQueryClient();
  const {
    isOpen: isSettingsOpen,
    open: openSettings,
    close: closeSettings,
  } = useSettingsModal();
  const { error: tempError, flash: flashTempError } = useTempError();

  const {
    data: vehiclesData = initialVehicles ?? [],
    isLoading: vehiclesLoading,
    error: vehiclesError,
  } = useQuery({
    queryKey: queryKeys.vehicles.all,
    queryFn: fetchVehiclesList,
    staleTime: 5 * 60 * 1000,
    initialData: initialVehicles,
    initialDataUpdatedAt,
  });

  const vehicles = vehiclesData ?? [];

  const isLoading = initialVehicles === undefined ? vehiclesLoading : false;

  const updatePercentsMutation = useMutation({
    // Serialize overlapping commits for the same vehicle so writes can never
    // land out of order (last-write-wins becomes last-call-wins).
    scope: { id: "vehicle-percents" },
    mutationFn: async ({
      vehicleId,
      currentPercent,
      targetPercent,
    }: {
      vehicleId: string;
      currentPercent: number;
      targetPercent: number;
    }) => {
      const ok = await apiPatchRaw(`/api/vehicles/${vehicleId}`, {
        currentPercent,
        targetPercent,
      });
      // apiPatchRaw never throws; surface failure so onError can roll back.
      if (!ok) throw new Error("Failed to update vehicle percents");
      return ok;
    },
    onMutate: async ({ vehicleId, currentPercent, targetPercent }) => {
      // Cancel in-flight refetches so a stale GET can't overwrite the cache,
      // then optimistically reflect the new percents so the gauge never reverts.
      await queryClient.cancelQueries({ queryKey: queryKeys.vehicles.all });
      const prevVehicles = queryClient.getQueryData<Vehicle[]>(
        queryKeys.vehicles.all,
      );
      if (prevVehicles) {
        queryClient.setQueryData<Vehicle[]>(
          queryKeys.vehicles.all,
          prevVehicles.map((v) =>
            v.id === vehicleId ? { ...v, currentPercent, targetPercent } : v,
          ),
        );
      }
      return { prevVehicles };
    },
    onError: (err, _vars, context) => {
      if (context?.prevVehicles) {
        queryClient.setQueryData(queryKeys.vehicles.all, context.prevVehicles);
      }
      flashTempError("Failed to save battery levels. Please try again.");
      console.error("Failed to update vehicle percents:", err);
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.vehicles.all });
    },
  });

  const updatePercents = useCallback(
    async (vehicleId: string, currentPct: number, targetPct: number) => {
      try {
        return await updatePercentsMutation.mutateAsync({
          vehicleId,
          currentPercent: currentPct,
          targetPercent: targetPct,
        });
      } catch {
        // Rollback + user-facing error handled in the mutation's onError.
        return false;
      }
    },
    [updatePercentsMutation],
  );

  const updateNotificationPrefsMutation = useMutation({
    mutationFn: async ({
      vehicleId,
      prefs,
    }: {
      vehicleId: string;
      prefs: {
        notifyChargeComplete?: boolean;
        notifyChargerOffline?: boolean;
        notifyMaintenanceOffline?: boolean;
      };
    }) => {
      const ok = await apiPatchRaw(`/api/vehicles/${vehicleId}`, prefs);
      if (!ok) throw new Error("Failed to update notification preferences");
      return ok;
    },
    onMutate: async ({ vehicleId, prefs }) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.vehicles.all });
      const prevVehicles = queryClient.getQueryData<Vehicle[]>(
        queryKeys.vehicles.all,
      );
      if (prevVehicles) {
        queryClient.setQueryData<Vehicle[]>(
          queryKeys.vehicles.all,
          prevVehicles.map((v) =>
            v.id === vehicleId ? { ...v, ...prefs } : v,
          ),
        );
      }
      return { prevVehicles };
    },
    onError: (_err, _vars, context) => {
      if (context?.prevVehicles) {
        queryClient.setQueryData(queryKeys.vehicles.all, context.prevVehicles);
      }
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.vehicles.all });
    },
  });

  const updateNotificationPrefs = useCallback(
    async (
      vehicleId: string,
      prefs: {
        notifyChargeComplete?: boolean;
        notifyChargerOffline?: boolean;
        notifyMaintenanceOffline?: boolean;
      },
    ) => {
      try {
        return await updateNotificationPrefsMutation.mutateAsync({
          vehicleId,
          prefs,
        });
      } catch {
        return false;
      }
    },
    [updateNotificationPrefsMutation],
  );

  return {
    vehicles,
    isLoading,
    error: vehiclesError,
    handleOpenSettings: openSettings,
    isSettingsOpen,
    closeSettings,
    tempError,
    setTempError: flashTempError,
    updatePercents,
    updateNotificationPrefs,
  };
}
