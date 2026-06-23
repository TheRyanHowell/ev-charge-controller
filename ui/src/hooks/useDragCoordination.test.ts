import { act } from "@/test-utils";
import { renderHook } from "@testing-library/react";
import { describe, it, expect } from "vitest";

import { useDragCoordination } from "./useDragCoordination";

describe("useDragCoordination", () => {
  it("starts with isDragging false", () => {
    const { result } = renderHook(() => useDragCoordination());
    expect(result.current.isDraggingRef.current).toBe(false);
  });

  it("sets isDragging true on drag start", () => {
    const { result } = renderHook(() => useDragCoordination());
    act(() => {
      result.current.onDragStart();
    });
    expect(result.current.isDraggingRef.current).toBe(true);
  });

  it("sets isDragging false on drag end", () => {
    const { result } = renderHook(() => useDragCoordination());
    act(() => {
      result.current.onDragStart();
    });
    act(() => {
      result.current.onDragEnd(50, 80);
    });
    expect(result.current.isDraggingRef.current).toBe(false);
  });

  it("onDragEnd is stable reference", () => {
    const { result } = renderHook(() => useDragCoordination());
    const first = result.current.onDragEnd;
    act(() => {
      result.current.onDragStart();
    });
    expect(result.current.onDragEnd).toBe(first);
  });

  it("onDragStart is stable reference", () => {
    const { result } = renderHook(() => useDragCoordination());
    const first = result.current.onDragStart;
    act(() => {
      result.current.onDragEnd(50, 80);
    });
    expect(result.current.onDragStart).toBe(first);
  });

  it("stays asserted after drag end while a commit is in flight", () => {
    const { result } = renderHook(() => useDragCoordination());
    act(() => {
      result.current.onDragStart();
      result.current.setCommitting(true);
    });
    // Pointer released, but the write is still in flight.
    act(() => {
      result.current.onDragEnd(50, 80);
    });
    expect(result.current.isDraggingRef.current).toBe(true);
    // Commit settles → guard releases.
    act(() => {
      result.current.setCommitting(false);
    });
    expect(result.current.isDraggingRef.current).toBe(false);
  });

  it("setCommitting alone (keyboard commit) gates and releases the guard", () => {
    const { result } = renderHook(() => useDragCoordination());
    act(() => {
      result.current.setCommitting(true);
    });
    expect(result.current.isDraggingRef.current).toBe(true);
    act(() => {
      result.current.setCommitting(false);
    });
    expect(result.current.isDraggingRef.current).toBe(false);
  });
});
