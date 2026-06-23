// OpenTelemetry instrumentation for Next.js
// Provides distributed tracing with console output for development
// Production can export to any OTEL-compatible backend (Jaeger, Grafana, etc.)

export function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    registerOtel();
  }
}

function registerOtel() {
  try {
    // Import OTEL only if available
    const otel = require("@vercel/otel");
    otel.init();
  } catch {
    // OTEL not installed, continue with console logging only
    console.debug("@vercel/otel not installed, using console logging");
  }
}
