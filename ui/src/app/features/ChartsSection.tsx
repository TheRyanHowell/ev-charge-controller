import CarbonIntensityChart from "@/components/CarbonIntensityChart";
import CurrentChart from "@/components/CurrentChart";
import ErrorBoundary from "@/components/ErrorBoundary";
import PowerChart from "@/components/PowerChart";
import SocChart from "@/components/SocChart";
import { PowerReading, SOCSnapshot } from "@/types/chart";

interface ChartsSectionProps {
  vehicleId?: string | null;
  shouldPoll: boolean;
  initialPowerReadings?: PowerReading[];
  initialSocSnapshots?: SOCSnapshot[];
}

function ChartCard({
  title,
  unit,
  tooltip,
  fallbackMessage,
  children,
}: {
  title: string;
  unit?: string;
  tooltip?: string;
  fallbackMessage: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex-1 min-w-0 rounded-xl bg-surface border border-border overflow-hidden flex flex-col">
      <div className="px-4 pt-4 pb-2">
        <span
          className="text-xs font-medium text-gray-500 uppercase tracking-wider"
          title={tooltip}
        >
          {title}
          {unit && <span className="normal-case"> {unit}</span>}
        </span>
      </div>
      <div className="flex-1 px-4 pb-4">
        <ErrorBoundary
          fallback={
            <div className="text-center py-8 text-red-400 text-sm">
              {fallbackMessage}
            </div>
          }
        >
          {children}
        </ErrorBoundary>
      </div>
    </div>
  );
}

export default function ChartsSection({
  vehicleId,
  shouldPoll,
  initialPowerReadings,
  initialSocSnapshots,
}: ChartsSectionProps) {
  const vid = vehicleId ?? undefined;
  return (
    <div className="mt-6 grid min-[1150px]:grid-cols-2 gap-6">
      <ChartCard
        title="Power Draw"
        unit="(kW)"
        tooltip="Electricity drawn from the charger over time, in kilowatts"
        fallbackMessage="Power chart unavailable"
      >
        <PowerChart
          vehicleId={vid}
          shouldPoll={shouldPoll}
          initialData={initialPowerReadings}
        />
      </ChartCard>
      <ChartCard
        title="State of Charge"
        unit="(%)"
        tooltip="Battery level over time, measured by the vehicle's battery management system"
        fallbackMessage="SOC chart unavailable"
      >
        <SocChart
          vehicleId={vid}
          shouldPoll={shouldPoll}
          initialData={initialSocSnapshots}
        />
      </ChartCard>
      <ChartCard
        title="Current Draw"
        unit="(A)"
        tooltip="Electrical current from the charger over time, in amps"
        fallbackMessage="Current chart unavailable"
      >
        <CurrentChart
          vehicleId={vid}
          shouldPoll={shouldPoll}
          initialData={initialPowerReadings}
        />
      </ChartCard>
      <ChartCard
        title="Carbon Intensity"
        unit="(gCO₂/kWh)"
        tooltip="Grid CO₂ per kilowatt-hour over time - lower means greener electricity"
        fallbackMessage="Carbon intensity chart unavailable"
      >
        <CarbonIntensityChart
          vehicleId={vid}
          shouldPoll={shouldPoll}
          initialData={initialPowerReadings}
        />
      </ChartCard>
    </div>
  );
}
