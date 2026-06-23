import { HistoryChargeSession, HistoryVehicle } from "@/lib/schemas";
import { getVehicleName } from "@/utils/history";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useHistoryDelete } from "./useHistoryDelete";
import { useHistorySessions } from "./useHistorySessions";
import { useHistoryVehicles } from "./useHistoryVehicles";

export function useHistory({
  initialVehicles,
  initialSessions,
  initialDate,
}: {
  initialVehicles?: HistoryVehicle[];
  initialSessions?: HistoryChargeSession[];
  initialDate?: string;
} = {}) {
  const [selectedVehicleId, setSelectedVehicleId] = useState<string | null>(
    null,
  );
  const [selectedDate, setSelectedDate] = useState<string>(
    initialDate ?? new Date().toISOString().split("T")[0] ?? "",
  );
  const [expandedSessionId, setExpandedSessionId] = useState<string | null>(
    null,
  );
  const [previousSessions, setPreviousSessions] = useState<
    HistoryChargeSession[] | undefined
  >(undefined);

  const sessionsRef = useRef<HistoryChargeSession[]>([]);

  const { vehicles, error: vehiclesError } = useHistoryVehicles({
    initialVehicles,
  });
  const { deleteSession, isDeleting, deleteError } = useHistoryDelete();

  const {
    sessions: querySessions,
    error: fetchError,
    loading: queryLoading,
    hasMore,
    isFetchingNextPage,
    isFetching,
    loadMore: loadMoreSessions,
  } = useHistorySessions(selectedVehicleId, selectedDate, {
    initialSessions,
  });

  useEffect(() => {
    if (querySessions) {
      sessionsRef.current = querySessions;
    }
  }, [querySessions]);

  const sessions = useMemo(() => {
    if (querySessions !== undefined) return querySessions;
    if (previousSessions && isFetching) return previousSessions;
    return [];
  }, [querySessions, isFetching, previousSessions]);

  const loading = queryLoading;

  const handleSetDate = useCallback((date: string) => {
    setPreviousSessions(sessionsRef.current);
    setSelectedDate(date);
  }, []);

  const loadMore = useCallback(() => {
    if (!hasMore || isFetchingNextPage) return;
    loadMoreSessions();
  }, [loadMoreSessions, hasMore, isFetchingNextPage]);

  const handleVehicleChange = useCallback((vehicleId: string) => {
    setPreviousSessions(sessionsRef.current);
    setSelectedVehicleId(vehicleId === "all" ? null : vehicleId);
    setExpandedSessionId(null);
  }, []);

  const toggleExpand = useCallback((sessionId: string) => {
    setExpandedSessionId((prev) => (prev === sessionId ? null : sessionId));
  }, []);

  const boundGetVehicleName = useCallback(
    (vehicleId: string) => getVehicleName(vehicles, vehicleId),
    [vehicles],
  );

  const isExpanded = useCallback(
    (sessionId: string) => expandedSessionId === sessionId,
    [expandedSessionId],
  );

  return {
    sessions,
    vehicles,
    selectedVehicleId,
    selectedDate,
    setSelectedDate: handleSetDate,
    loading,
    error: fetchError,
    vehiclesError,
    hasMore,
    isDeleting,
    deleteError,
    loadMore,
    handleVehicleChange,
    toggleExpand,
    deleteSession,
    getVehicleName: boundGetVehicleName,
    isExpanded,
  };
}
