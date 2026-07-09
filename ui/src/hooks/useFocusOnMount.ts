import { useCallback } from "react";

/**
 * Callback ref that focuses the element when it mounts. Unlike the
 * `autoFocus` JSX prop (flagged by jsx-a11y/no-autofocus, since it commonly
 * disorients screen reader users when it fires on page load), this is meant
 * for elements that appear as a direct result of a user action - e.g.
 * entering inline-edit mode - and the returned function is stable across
 * re-renders so it only fires on actual mount, not every render.
 */
export function useFocusOnMount<T extends HTMLElement>() {
  return useCallback((element: T | null) => {
    element?.focus();
  }, []);
}
