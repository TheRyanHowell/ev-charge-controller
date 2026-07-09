import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

import { useThemeStore, watchSystemTheme } from "./themeStore";

const THEME_STORAGE_KEY = "theme";

describe("themeStore", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove("dark");
    useThemeStore.setState({ theme: "light" });
  });

  afterEach(() => {
    document.documentElement.classList.remove("dark");
  });

  describe("initial state", () => {
    it("defaults to light when documentElement has no dark class", async () => {
      document.documentElement.classList.remove("dark");
      vi.resetModules();
      const { useThemeStore: freshStore } = await import("./themeStore");
      expect(freshStore.getState().theme).toBe("light");
    });

    it("defaults to dark when documentElement has the dark class", async () => {
      document.documentElement.classList.add("dark");
      vi.resetModules();
      const { useThemeStore: freshStore } = await import("./themeStore");
      expect(freshStore.getState().theme).toBe("dark");
    });
  });

  describe("toggleTheme", () => {
    it("flips theme from light to dark", () => {
      useThemeStore.getState().toggleTheme();
      expect(useThemeStore.getState().theme).toBe("dark");
    });

    it("flips theme from dark to light", () => {
      document.documentElement.classList.add("dark");
      useThemeStore.setState({ theme: "dark" });
      useThemeStore.getState().toggleTheme();
      expect(useThemeStore.getState().theme).toBe("light");
    });

    it("adds the dark class to documentElement when toggling to dark", () => {
      useThemeStore.getState().toggleTheme();
      expect(document.documentElement.classList.contains("dark")).toBe(true);
    });

    it("removes the dark class from documentElement when toggling to light", () => {
      document.documentElement.classList.add("dark");
      useThemeStore.setState({ theme: "dark" });
      useThemeStore.getState().toggleTheme();
      expect(document.documentElement.classList.contains("dark")).toBe(false);
    });

    it("persists dark theme to localStorage", () => {
      useThemeStore.getState().toggleTheme();
      expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark");
    });

    it("persists light theme to localStorage", () => {
      document.documentElement.classList.add("dark");
      useThemeStore.setState({ theme: "dark" });
      useThemeStore.getState().toggleTheme();
      expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("light");
    });
  });

  describe("watchSystemTheme", () => {
    function mockMatchMedia() {
      let listener: ((e: MediaQueryListEvent) => void) | null = null;
      const media = {
        matches: false,
        media: "(prefers-color-scheme: dark)",
        addEventListener: (
          _event: string,
          cb: (e: MediaQueryListEvent) => void,
        ) => {
          listener = cb;
        },
        removeEventListener: () => {
          listener = null;
        },
      };
      vi.spyOn(window, "matchMedia").mockReturnValue(
        media as unknown as MediaQueryList,
      );
      return {
        trigger: (matches: boolean) =>
          listener?.({ matches } as MediaQueryListEvent),
      };
    }

    afterEach(() => {
      vi.restoreAllMocks();
    });

    it("switches to dark when the OS preference changes and no explicit choice is stored", () => {
      const { trigger } = mockMatchMedia();
      const stop = watchSystemTheme();

      trigger(true);

      expect(useThemeStore.getState().theme).toBe("dark");
      expect(document.documentElement.classList.contains("dark")).toBe(true);
      stop();
    });

    it("switches to light when the OS preference changes and no explicit choice is stored", () => {
      document.documentElement.classList.add("dark");
      useThemeStore.setState({ theme: "dark" });
      const { trigger } = mockMatchMedia();
      const stop = watchSystemTheme();

      trigger(false);

      expect(useThemeStore.getState().theme).toBe("light");
      expect(document.documentElement.classList.contains("dark")).toBe(false);
      stop();
    });

    it("ignores OS preference changes once the user has made an explicit choice", () => {
      localStorage.setItem(THEME_STORAGE_KEY, "light");
      const { trigger } = mockMatchMedia();
      const stop = watchSystemTheme();

      trigger(true);

      expect(useThemeStore.getState().theme).toBe("light");
      stop();
    });
  });
});
