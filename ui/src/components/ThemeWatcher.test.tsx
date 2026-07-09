import * as themeStore from "@/stores/themeStore";
import { render, cleanup } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";

import ThemeWatcher from "./ThemeWatcher";

describe("ThemeWatcher", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("starts watching the system theme on mount", () => {
    const unsubscribe = vi.fn();
    const watchSpy = vi
      .spyOn(themeStore, "watchSystemTheme")
      .mockReturnValue(unsubscribe);

    render(<ThemeWatcher />);

    expect(watchSpy).toHaveBeenCalledTimes(1);
  });

  it("stops watching on unmount", () => {
    const unsubscribe = vi.fn();
    vi.spyOn(themeStore, "watchSystemTheme").mockReturnValue(unsubscribe);

    const { unmount } = render(<ThemeWatcher />);
    unmount();

    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });

  it("renders nothing", () => {
    vi.spyOn(themeStore, "watchSystemTheme").mockReturnValue(() => {});

    const { container } = render(<ThemeWatcher />);

    expect(container).toBeEmptyDOMElement();
  });
});
