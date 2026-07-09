"use client";

import { watchSystemTheme } from "@/stores/themeStore";
import { useEffect } from "react";

/** Mounted once at the app root to keep the theme synced with OS changes. */
export default function ThemeWatcher() {
  useEffect(() => watchSystemTheme(), []);
  return null;
}
