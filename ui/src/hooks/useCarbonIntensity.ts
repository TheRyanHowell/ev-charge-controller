import type { CarbonIntensity } from "@/lib/schemas";

import { apiGetSingle } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import { CarbonIntensitySchema } from "@/lib/schemas";
import { useQuery } from "@tanstack/react-query";

const CarbonIntensityPollIntervalMs = 60 * 1000;

export function useCarbonIntensity(initialData?: CarbonIntensity | null): {
  carbonIntensity: CarbonIntensity | null;
} {
  const { data } = useQuery({
    queryKey: queryKeys.carbonIntensity.current,
    queryFn: () => apiGetSingle("/api/carbon-intensity", CarbonIntensitySchema),
    refetchInterval: CarbonIntensityPollIntervalMs,
    refetchOnMount: false,
    initialData: initialData,
    staleTime: CarbonIntensityPollIntervalMs,
  });

  return { carbonIntensity: data ?? null };
}
