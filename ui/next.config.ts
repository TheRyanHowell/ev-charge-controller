import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // No rewrites/proxies needed - API route handlers in /app/api/ proxy to the Go backend
  // via fetch(${API_URL}/...). See docs/TESTING.md for the full local dev stack.
  // Allow E2E tests (running in separate Docker container) to access Turbopack HMR WebSocket
  allowedDevOrigins: ["ui"],
  async headers() {
    return [
      {
        // Service worker must never be served stale - browsers cache it aggressively
        source: "/sw.js",
        headers: [
          { key: "Cache-Control", value: "no-cache, no-store, must-revalidate" },
          { key: "Content-Type", value: "application/javascript; charset=utf-8" },
        ],
      },
    ];
  },
};

export default nextConfig;
