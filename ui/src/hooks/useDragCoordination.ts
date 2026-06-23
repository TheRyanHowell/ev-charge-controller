import { useRef, useCallback } from "react";

/**
 * Coordinates the "is the user actively setting markers" signal that gates
 * inbound gauge syncs (session polling, prop→store sync). The signal stays
 * asserted while the pointer is down AND while a percent commit is in flight,
 * so a poll/refetch that lands during the debounce + request window cannot
 * clobber the value the user just set.
 */
export function useDragCoordination() {
  // Public signal read by consumers (e.g. useSessionPolling).
  const isDraggingRef = useRef(false);
  // Private component signals OR'd into isDraggingRef.
  const draggingRef = useRef(false);
  const committingRef = useRef(false);

  const onDragStart = useCallback(() => {
    draggingRef.current = true;
    isDraggingRef.current = draggingRef.current || committingRef.current;
  }, []);

  const onDragEnd = useCallback((_current: number, _target: number) => {
    draggingRef.current = false;
    isDraggingRef.current = draggingRef.current || committingRef.current;
  }, []);

  // Held by an in-flight commit so inbound syncs stay suppressed until the
  // write settles (covers the post-pointer-up debounce + request window).
  const setCommitting = useCallback((committing: boolean) => {
    committingRef.current = committing;
    isDraggingRef.current = draggingRef.current || committingRef.current;
  }, []);

  return { isDraggingRef, onDragStart, onDragEnd, setCommitting };
}
