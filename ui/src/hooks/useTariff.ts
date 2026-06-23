import type { TariffSettings } from "@/lib/schemas";

import { apiGetSingle, apiPut } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import { TariffSettingsSchema } from "@/lib/schemas";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useCallback } from "react";

async function fetchTariffSettings(): Promise<TariffSettings | null> {
  return await apiGetSingle("/api/tariff-settings", TariffSettingsSchema);
}

/**
 * useTariff exposes the current user's electricity tariff (base rate + off-peak
 * windows) and a mutation to replace it. The query is enabled lazily so the
 * Settings modal only fetches the tariff when it is opened.
 */
export function useTariff({ enabled = true }: { enabled?: boolean } = {}) {
  const {
    data: settings = null,
    isLoading,
    error,
  } = useQuery({
    queryKey: queryKeys.tariff.settings,
    queryFn: fetchTariffSettings,
    staleTime: 5 * 60 * 1000,
    enabled,
  });

  const updateMutation = useMutation({
    mutationFn: async (next: TariffSettings) => {
      return await apiPut("/api/tariff-settings", TariffSettingsSchema, {
        baseRatePence: next.baseRatePence,
        offPeakWindows: next.offPeakWindows,
      });
    },
    // No onSuccess cache update — the Settings modal is transient and the
    // 5-minute staleTime ensures data is fresh on next open. Avoiding
    // setQueryData here prevents re-renders that can race with local state.
  });

  const updateTariff = useCallback(
    (next: TariffSettings) => updateMutation.mutateAsync(next),
    [updateMutation],
  );

  return {
    settings,
    isLoading,
    error,
    updateTariff,
    isSaving: updateMutation.isPending,
    saveError: updateMutation.error,
  };
}
