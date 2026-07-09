import { renderHook } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import { useFocusOnMount } from "./useFocusOnMount";

describe("useFocusOnMount", () => {
  it("focuses the element it's attached to", () => {
    const { result } = renderHook(() => useFocusOnMount<HTMLInputElement>());
    const input = document.createElement("input");
    const focusSpy = vi.spyOn(input, "focus");

    result.current(input);

    expect(focusSpy).toHaveBeenCalledOnce();
  });

  it("does nothing when the ref is detached (element is null)", () => {
    const { result } = renderHook(() => useFocusOnMount<HTMLInputElement>());
    expect(() => result.current(null)).not.toThrow();
  });

  it("returns a stable callback across re-renders so it only fires on actual mount/unmount", () => {
    const { result, rerender } = renderHook(() =>
      useFocusOnMount<HTMLInputElement>(),
    );
    const first = result.current;
    rerender();
    expect(result.current).toBe(first);
  });
});
