"use client";

import { useEffect } from "react";

// Debounce so a burst of DOM mutations (e.g. a re-render touching many
// nodes) triggers one scan, not one per mutation.
const ScanDebounceMs = 1000;

/** Dev-only: runs axe-core against the live DOM and logs WCAG violations to the console. */
export default function AxeAuditor() {
  useEffect(() => {
    if (process.env.NODE_ENV === "production") return;

    let cancelled = false;
    let debounceTimer: ReturnType<typeof setTimeout> | undefined;
    let observer: MutationObserver | undefined;

    void import("axe-core").then((axe) => {
      if (cancelled) return;

      const runScan = () => {
        void axe.default.run(document, (err, results) => {
          if (err) {
            console.error("axe-core scan failed:", err);
            return;
          }
          if (results.violations.length > 0) {
            console.warn("axe-core found WCAG violations:", results.violations);
          }
        });
      };

      // Audit once after the initial render settles, then on subsequent
      // DOM changes so the console stays in sync while navigating/toggling.
      debounceTimer = setTimeout(runScan, ScanDebounceMs);

      observer = new MutationObserver(() => {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(runScan, ScanDebounceMs);
      });
      observer.observe(document.body, {
        childList: true,
        subtree: true,
        attributes: true,
      });
    });

    return () => {
      cancelled = true;
      clearTimeout(debounceTimer);
      observer?.disconnect();
    };
  }, []);
  return null;
}
