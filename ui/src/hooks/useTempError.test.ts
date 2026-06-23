import { customRenderHook as renderHook, act } from "@/test-utils";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import { useTempError } from "./useTempError";

describe("useTempError", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("starts with null error", () => {
    const { result } = renderHook(() => useTempError());
    expect(result.current.error).toBeNull();
  });

  it("sets error message on flash", () => {
    const { result } = renderHook(() => useTempError());
    act(() => {
      result.current.flash("Test error");
    });
    expect(result.current.error).toBe("Test error");
  });

  it("auto-clears error after default timeout (5000ms)", async () => {
    const { result } = renderHook(() => useTempError());
    act(() => {
      result.current.flash("Auto dismiss");
    });
    expect(result.current.error).toBe("Auto dismiss");

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });

    expect(result.current.error).toBeNull();
  });

  it("clears pending timer when flashing new error", async () => {
    const { result } = renderHook(() => useTempError());
    act(() => {
      result.current.flash("First");
    });

    await act(async () => {
      vi.advanceTimersByTime(2000);
    });

    act(() => {
      result.current.flash("Second");
    });

    await act(async () => {
      vi.advanceTimersByTime(2000);
    });

    expect(result.current.error).toBe("Second");

    await act(async () => {
      vi.advanceTimersByTime(3000);
    });

    expect(result.current.error).toBeNull();
  });

  it("accepts custom timeout", async () => {
    const { result } = renderHook(() => useTempError(3000));
    act(() => {
      result.current.flash("Custom timeout");
    });

    await act(async () => {
      vi.advanceTimersByTime(2999);
    });

    expect(result.current.error).toBe("Custom timeout");

    await act(async () => {
      vi.advanceTimersByTime(1);
    });

    expect(result.current.error).toBeNull();
  });

  it("cleans up timer on unmount", async () => {
    const { result, unmount } = renderHook(() => useTempError());
    act(() => {
      result.current.flash("Should not leak");
    });
    expect(result.current.error).toBe("Should not leak");
    unmount();

    // Advancing timers after unmount should not cause errors
    await act(async () => {
      vi.advanceTimersByTime(10000);
    });
  });
});
