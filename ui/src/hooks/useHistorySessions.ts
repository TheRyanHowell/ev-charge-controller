import { apiGet } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import {
  HistoryChargeSession,
  HistoryChargeSessionSchema,
} from "@/lib/schemas";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { InfiniteData } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef } from "react";

const DEFAULT_PAGE_SIZE = 50;

export function useHistorySessions(
  vehicleId: string | null,
  date: string,
  {
    initialSessions,
  }: {
    initialSessions?: HistoryChargeSession[];
  } = {},
) {
  const queryClient = useQueryClient();
  const queryKey = useMemo(
    () => queryKeys.history.sessions(vehicleId, date),
    [vehicleId, date],
  );

  const initialApplied = useRef(false);

  // Only seed initial sessions on first mount. Subsequent filter changes
  // must trigger a fresh fetch, not replay stale server data.
  useEffect(() => {
    if (initialApplied.current || initialSessions === undefined) return;
    initialApplied.current = true;
    const data: InfiniteData<HistoryChargeSession[], unknown> = {
      pages: [initialSessions],
      pageParams: [0],
    };
    queryClient.setQueryData(queryKey, data);
  }, [queryClient, initialSessions, queryKey]);

  const query = useInfiniteQuery({
    queryKey,
    queryFn: async ({ pageParam }: { pageParam: number }) => {
      const params = new URLSearchParams({
        date,
        limit: String(DEFAULT_PAGE_SIZE),
        offset: String(pageParam),
      });
      if (vehicleId) {
        params.set("vehicleId", vehicleId);
      }
      return apiGet(
        `/api/history?${params.toString()}`,
        HistoryChargeSessionSchema,
      );
    },
    initialPageParam: 0,
    getNextPageParam: (
      lastPage: HistoryChargeSession[],
      _allPages: HistoryChargeSession[][],
      lastPageParam: number,
    ) =>
      lastPage.length >= DEFAULT_PAGE_SIZE
        ? lastPageParam + DEFAULT_PAGE_SIZE
        : undefined,
    staleTime: 10_000,
    retry: false,
    select: (data) => data.pages.flatMap((p) => p),
  });

  const sessions = useMemo(() => {
    return query.data;
  }, [query.data]);
  const error = query.error
    ? "Unable to connect to the API server. Is the backend running?"
    : null;
  const loading =
    initialSessions === undefined
      ? query.isLoading || query.isFetchingNextPage
      : false;
  const hasMore = query.hasNextPage ?? false;
  const { fetchNextPage, isFetchingNextPage } = query;

  const loadMore = useCallback(() => {
    if (!hasMore || isFetchingNextPage) return;
    fetchNextPage();
  }, [fetchNextPage, hasMore, isFetchingNextPage]);

  return {
    sessions,
    error,
    loading,
    hasMore,
    isFetchingNextPage,
    isFetching: query.isFetching,
    loadMore,
  };
}
