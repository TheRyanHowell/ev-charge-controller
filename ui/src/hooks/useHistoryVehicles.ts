import { apiGet } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import { HistoryVehicle, HistoryVehicleSchema } from "@/lib/schemas";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo } from "react";

export function useHistoryVehicles({
  initialVehicles,
}: {
  initialVehicles?: HistoryVehicle[];
} = {}) {
  const queryClient = useQueryClient();

  useEffect(() => {
    if (initialVehicles !== undefined) {
      queryClient.setQueryData(queryKeys.history.vehicles, initialVehicles);
    }
  }, [queryClient, initialVehicles]);

  const query = useQuery({
    queryKey: queryKeys.history.vehicles,
    queryFn: () => apiGet("/api/vehicles", HistoryVehicleSchema),
    staleTime: 5 * 60 * 1000,
    retry: false,
  });

  const vehicles = useMemo(() => query.data ?? [], [query.data]);
  const error = query.error ? "Failed to load vehicle list" : null;
  const loading = initialVehicles === undefined ? query.isLoading : false;

  return { vehicles, error, loading };
}
