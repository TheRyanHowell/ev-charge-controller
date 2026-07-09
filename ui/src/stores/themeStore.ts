import { create } from "zustand";

export type Theme = "light" | "dark";

interface ThemeState {
  theme: Theme;
  toggleTheme: () => void;
}

const THEME_STORAGE_KEY = "theme";
const DARK_CLASS = "dark";

function getInitialTheme(): Theme {
  if (typeof document === "undefined") return "light";
  return document.documentElement.classList.contains(DARK_CLASS)
    ? "dark"
    : "light";
}

function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle(DARK_CLASS, theme === "dark");
}

export const useThemeStore = create<ThemeState>()((set, get) => ({
  theme: getInitialTheme(),

  toggleTheme: () => {
    const next: Theme = get().theme === "dark" ? "light" : "dark";
    applyTheme(next);
    localStorage.setItem(THEME_STORAGE_KEY, next);
    set({ theme: next });
  },
}));

/**
 * Keeps the theme following the OS setting for as long as the user hasn't
 * made an explicit choice via toggleTheme (i.e. no localStorage entry yet).
 * Returns an unsubscribe function.
 */
export function watchSystemTheme(): () => void {
  if (typeof window === "undefined") return () => {};

  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const handleChange = (e: MediaQueryListEvent) => {
    if (localStorage.getItem(THEME_STORAGE_KEY) !== null) return;
    const next: Theme = e.matches ? "dark" : "light";
    applyTheme(next);
    useThemeStore.setState({ theme: next });
  };

  media.addEventListener("change", handleChange);
  return () => media.removeEventListener("change", handleChange);
}
