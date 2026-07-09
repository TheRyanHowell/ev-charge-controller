"use client";

import React from "react";

import CarbonIntensityChart from "./CarbonIntensityChart";
import CurrentChart from "./CurrentChart";
import ErrorBoundary from "./ErrorBoundary";
import PowerChart from "./PowerChart";
import SocChart from "./SocChart";

function ChartSection({
  title,
  tooltip,
  fallback,
  children,
}: {
  title: string;
  tooltip?: string;
  fallback: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <h3 className="text-sm font-medium text-fg-muted mb-2" title={tooltip}>
        {title}
      </h3>
      <ErrorBoundary
        fallback={
          <div className="text-center py-4 text-danger text-sm">{fallback}</div>
        }
      >
        {children}
      </ErrorBoundary>
    </div>
  );
}

export default function SessionDetail({ sessionId }: { sessionId: string }) {
  return (
    <div className="pt-2 space-y-2">
      <ChartSection
        title="Power Draw (kW)"
        tooltip="Electricity drawn from the charger over time, in kilowatts"
        fallback="Power chart unavailable"
      >
        <PowerChart sessionId={sessionId} />
      </ChartSection>
      <ChartSection
        title="State of Charge (%)"
        tooltip="Battery level over time, measured by the vehicle's battery management system"
        fallback="SOC chart unavailable"
      >
        <SocChart sessionId={sessionId} />
      </ChartSection>
      <ChartSection
        title="Current Draw (A)"
        tooltip="Electrical current from the charger over time, in amps"
        fallback="Current chart unavailable"
      >
        <CurrentChart sessionId={sessionId} />
      </ChartSection>
      <ChartSection
        title="Carbon Intensity (gCO₂/kWh)"
        tooltip="Grid CO₂ per kilowatt-hour over time - lower means greener electricity"
        fallback="Carbon intensity chart unavailable"
      >
        <CarbonIntensityChart sessionId={sessionId} />
      </ChartSection>
    </div>
  );
}
