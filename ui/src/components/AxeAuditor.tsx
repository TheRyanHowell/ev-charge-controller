"use client";

import { useEffect } from "react";

/** Dev-only: runs axe-core against the live DOM and logs WCAG violations to the console. */
export default function AxeAuditor() {
  useEffect(() => {
    if (process.env.NODE_ENV === "production") return;
    void Promise.all([
      import("react"),
      import("react-dom"),
      import("@axe-core/react"),
    ]).then(([React, ReactDOM, axe]) => {
      axe.default(React, ReactDOM, 1000);
    });
  }, []);
  return null;
}
