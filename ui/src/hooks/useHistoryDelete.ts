import { apiDelete } from "@/lib/api";
import { queryKeys } from "@/lib/queryKeys";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

export function useHistoryDelete() {
  const queryClient = useQueryClient();

  const mutation = useMutation({
    mutationFn: async (sessionId: string) => {
      await apiDelete(`/api/charge-sessions/${encodeURIComponent(sessionId)}`);
      return sessionId;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.history.allSessions,
      });
    },
  });

  const deleteSession = useCallback(
    async (sessionId: string): Promise<boolean> => {
      try {
        await mutation.mutateAsync(sessionId);
        return true;
      } catch (err) {
        console.error("Failed to delete session:", err);
        return false;
      }
    },
    [mutation],
  );

  return {
    deleteSession,
    isDeleting: mutation.isPending,
    deleteError: mutation.error,
  };
}
