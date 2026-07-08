import type { Schedule } from "@/lib/schemas";

import { apiGetSingle, apiPatch } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import { ScheduleSchema } from "@/lib/schemas";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

export interface SchedulePayload {
  type: "daily" | "carbon_aware";
  time?: string;
  windowStart?: string;
  windowEnd?: string;
  readyBy?: string;
  twoStage?: boolean;
  enabled: boolean;
}

async function fetchScheduleData(plugId: string): Promise<Schedule | null> {
  return await apiGetSingle(`/api/plugs/${plugId}/schedule`, ScheduleSchema);
}

export function useSchedule(
  plugId: string | null,
  initialSchedule?: Schedule | null,
  renderTimeMs?: number,
) {
  const queryClient = useQueryClient();

  const queryKey = plugId ? queryKeys.plugs.schedule(plugId) : null;

  const {
    data: schedule = null,
    isLoading,
    error,
  } = useQuery({
    queryKey: queryKey ?? queryKeys.plugs.all,
    queryFn: () => (plugId ? fetchScheduleData(plugId) : Promise.resolve(null)),
    enabled: !!plugId,
    staleTime: 5 * 60 * 1000,
    initialData: initialSchedule,
    initialDataUpdatedAt: renderTimeMs,
  });

  const saveMutation = useMutation({
    mutationFn: (payload: SchedulePayload) => {
      if (!plugId) return Promise.reject(new Error("No plug selected"));
      return apiPatch(
        `/api/plugs/${plugId}/schedule`,
        ScheduleSchema,
        payload as unknown as Record<string, unknown>,
      );
    },
    onSuccess: (data) => {
      if (queryKey) {
        queryClient.setQueryData(queryKey, data);
      }
    },
    onError: (err) => {
      console.error(
        "Schedule save error:",
        err instanceof Error ? err.message : String(err),
      );
    },
  });

  const saveSchedule = async (
    payload: SchedulePayload,
  ): Promise<Schedule | null> => {
    try {
      return await saveMutation.mutateAsync(payload);
    } catch (err) {
      console.error(
        "Schedule save failed:",
        err instanceof Error ? err.message : String(err),
      );
      return null;
    }
  };

  return {
    schedule,
    isLoading,
    error,
    saveSchedule,
    isSaving: saveMutation.isPending,
    saveError: saveMutation.error,
  };
}
