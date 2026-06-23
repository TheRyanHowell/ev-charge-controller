import { act } from "@/test-utils";
import { renderHook } from "@testing-library/react";
import { describe, it, expect } from "vitest";

import { useSettingsModal } from "./useSettingsModal";

describe("useSettingsModal", () => {
  it("starts closed", () => {
    const { result } = renderHook(() => useSettingsModal());
    expect(result.current.isOpen).toBe(false);
  });

  it("opens on open", () => {
    const { result } = renderHook(() => useSettingsModal());
    act(() => {
      result.current.open();
    });
    expect(result.current.isOpen).toBe(true);
  });

  it("closes on close", () => {
    const { result } = renderHook(() => useSettingsModal());
    act(() => {
      result.current.open();
    });
    act(() => {
      result.current.close();
    });
    expect(result.current.isOpen).toBe(false);
  });

  it("close is safe when already closed", () => {
    const { result } = renderHook(() => useSettingsModal());
    act(() => {
      result.current.close();
    });
    expect(result.current.isOpen).toBe(false);
  });
});
