import type { Plug, ProvisioningResult } from "@/lib/schemas";

import { apiGet, apiPost, apiPatch, apiPatchRaw, apiDelete } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import { PlugSchema, ProvisioningResultSchema } from "@/lib/schemas";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, useCallback, useEffect } from "react";

const SELECTED_VEHICLE_COOKIE = "selected_vehicle_id";

async function fetchPlugs(): Promise<Plug[]> {
  return await apiGet("/api/plugs", PlugSchema);
}

async function togglePlugPower(plugId: string, on: boolean): Promise<Plug> {
  return apiPatch(`/api/plugs/${plugId}/power`, PlugSchema, { on });
}

export function usePlug(
  initialPlugs?: Plug[],
  initialDataUpdatedAt?: number,
  initialSelectedVehicleId?: string | null,
) {
  const queryClient = useQueryClient();
  const [selectedVehicleId, setSelectedVehicleId] = useState<string | null>(
    initialSelectedVehicleId ?? null,
  );

  // Re-initialize selectedVehicleId when it's stale (e.g. after DB reset).
  // Default to the vehicle of the first plug if nothing is selected.
  useEffect(() => {
    if (initialPlugs !== undefined && initialPlugs.length > 0) {
      const vehicleIds = [
        ...new Set(
          initialPlugs
            .map((p) => p.vehicleId)
            .filter((id): id is string => !!id),
        ),
      ];
      setSelectedVehicleId((currentId) => {
        if (currentId && vehicleIds.includes(currentId)) return currentId;
        return vehicleIds[0] ?? null;
      });
    }
  }, [initialPlugs]);

  const {
    data: plugsData = initialPlugs ?? [],
    isLoading,
    error,
  } = useQuery({
    queryKey: queryKeys.plugs.all,
    queryFn: fetchPlugs,
    staleTime: 60 * 1000,
    initialData: initialPlugs,
    initialDataUpdatedAt: initialDataUpdatedAt ?? 0,
  });

  const plugs = plugsData ?? [];

  const selectVehicle = useCallback((vehicleId: string) => {
    setSelectedVehicleId(vehicleId);
    document.cookie = `${SELECTED_VEHICLE_COOKIE}=${vehicleId}; path=/; max-age=31536000; SameSite=Lax`;
  }, []);

  const createMutation = useMutation({
    mutationFn: (vars: {
      name: string;
      mqttTopic?: string;
      vehicleId?: string;
      type?: "charging" | "maintenance";
    }) => apiPost("/api/plugs", ProvisioningResultSchema, vars),
    onSuccess: (result: ProvisioningResult) => {
      queryClient.setQueryData(queryKeys.plugs.all, (old: Plug[] = []) => [
        ...old,
        result.plug,
      ]);
      // Auto-select the vehicle of the newly created plug.
      if (result.plug.vehicleId) {
        selectVehicle(result.plug.vehicleId);
      }
    },
  });

  const updateMutation = useMutation({
    mutationFn: (vars: {
      plugId: string;
      name?: string;
      vehicleId?: string | null;
    }) => {
      const { plugId, ...body } = vars;
      return apiPatchRaw(`/api/plugs/${plugId}`, body);
    },
    onSuccess: (_result, vars) => {
      queryClient.setQueryData(queryKeys.plugs.all, (old: Plug[] = []) =>
        old.map((p) =>
          p.id === vars.plugId
            ? {
                ...p,
                ...(vars.name !== undefined && { name: vars.name }),
                ...(vars.vehicleId !== undefined && {
                  vehicleId: vars.vehicleId,
                }),
              }
            : p,
        ),
      );
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (plugId: string) => apiDelete(`/api/plugs/${plugId}`),
    onSuccess: (_result, deletedPlugId) => {
      queryClient.setQueryData(queryKeys.plugs.all, (old: Plug[] = []) =>
        old.filter((p) => p.id !== deletedPlugId),
      );
    },
  });

  const togglePowerMutation = useMutation({
    mutationFn: (vars: { plugId: string; on: boolean }) =>
      togglePlugPower(vars.plugId, vars.on),
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.plugs.all });
      const previous = queryClient.getQueryData<Plug[]>(queryKeys.plugs.all);
      queryClient.setQueryData(queryKeys.plugs.all, (old: Plug[] = []) =>
        old.map((p) => (p.id === vars.plugId ? { ...p, powerOn: vars.on } : p)),
      );
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(queryKeys.plugs.all, context.previous);
      }
    },
  });

  return {
    plugs,
    selectedVehicleId,
    selectVehicle,
    isLoading,
    error,
    createPlug: createMutation.mutateAsync,
    isCreating: createMutation.isPending,
    updatePlug: updateMutation.mutateAsync,
    deletePlug: deleteMutation.mutateAsync,
    toggleMaintenancePower: togglePowerMutation.mutateAsync,
    isTogglingPower: togglePowerMutation.isPending,
  };
}
