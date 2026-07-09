import type { CarbonIntensity, Plug, Schedule, Vehicle } from "@/lib/schemas";

import ErrorBoundary from "@/components/ErrorBoundary";
import SpeedometerGauge from "@/components/SpeedometerGauge";
import StatsPanel from "@/components/StatsPanel";

interface GaugeState {
  currentPercent: number;
  targetPercent: number;
  startPercent: number | null;
  status:
    | "idle"
    | "charging"
    | "pending"
    | "conditioning"
    | "holding"
    | "error";
  /** HH:MM best guess for when a holding session will resume. */
  estimatedResumeTime?: string | null;
}

interface SessionState {
  isChargingOrPending: boolean;
  isActionPending: boolean;
  sessionStartTime: number | null;
  renderTimeMs?: number;
}

interface Telemetry {
  powerDraw: number;
  energyAddedKwh: number | null;
  voltage: number | null;
  current: number | null;
}

interface Handlers {
  onStartCharging: () => void;
  onStopCharging: () => void;
  onDragStart: () => void;
  onChargeDragEnd: (current: number, target: number) => void;
  clearError: () => void;
  handleTargetChargeUpdate: (current: number, target: number) => void;
  updatePercents: (vehicleId: string, current: number, target: number) => void;
}

export interface ChargeControlProps {
  selectedVehicle: Vehicle | null;
  gauge: GaugeState;
  session: SessionState;
  telemetry: Telemetry;
  errorMessage: string | null;
  tasmotaConnected: boolean | null;
  carbonIntensity: CarbonIntensity | null;
  isActive: boolean;
  handlers: Handlers;
  schedule?: Schedule | null;
  onOpenSchedule?: () => void;
  /** Electricity rate (pence/kWh) in effect now, for live charging cost. */
  costPerKwh?: number;
  /** 12V maintenance plug for the selected vehicle, if any. */
  maintenancePlug?: Plug | null;
  onToggleMaintenance?: () => void;
  isMaintenancePending?: boolean;
}

export default function ChargeControl({
  selectedVehicle,
  gauge,
  session,
  telemetry,
  errorMessage,
  tasmotaConnected,
  carbonIntensity,
  isActive,
  handlers,
  schedule,
  onOpenSchedule,
  costPerKwh,
  maintenancePlug,
  onToggleMaintenance,
  isMaintenancePending,
}: ChargeControlProps) {
  // Single commit point for marker changes. Fired once per gesture
  // (pointer drag end or debounced keyboard edit) by SpeedometerGauge -
  // never on live, intermediate values during a drag.
  const handleDragEnd = (current: number, target: number) => {
    handlers.onChargeDragEnd(current, target);
    handlers.clearError();
    if (isActive) {
      handlers.handleTargetChargeUpdate(current, target);
    } else if (selectedVehicle) {
      handlers.updatePercents(selectedVehicle.id, current, target);
    }
  };

  return (
    <div className="flex flex-col min-[1150px]:flex-row min-[1150px]:items-center gap-6">
      <div className="flex-none w-full min-[1150px]:w-[540px]">
        <ErrorBoundary
          fallback={
            <div className="text-center py-16 text-danger">
              Gauge unavailable
            </div>
          }
        >
          <SpeedometerGauge
            startPercent={gauge.startPercent ?? gauge.currentPercent}
            currentPercent={gauge.currentPercent}
            targetPercent={gauge.targetPercent}
            status={gauge.status}
            onStartStop={() => {
              if (session.isChargingOrPending) {
                handlers.onStopCharging();
              } else {
                handlers.onStartCharging();
              }
            }}
            isActionPending={session.isActionPending}
            onDragStart={handlers.onDragStart}
            onDragEnd={handleDragEnd}
            tasmotaConnected={tasmotaConnected}
            schedule={schedule}
            onOpenSchedule={onOpenSchedule}
            maintenance={
              maintenancePlug
                ? {
                    powerOn: maintenancePlug.powerOn,
                    online: maintenancePlug.online,
                  }
                : null
            }
            onToggleMaintenance={onToggleMaintenance}
            isMaintenancePending={isMaintenancePending}
            estimatedResumeTime={gauge.estimatedResumeTime}
          />
        </ErrorBoundary>
      </div>

      <div
        className="hidden min-[1150px]:block w-[540px] flex-shrink-0 flex justify-center"
        {...(telemetry.powerDraw > 0 && { "data-testid": "power-draw" })}
      >
        <div className="max-w-[520px] mx-auto" data-testid="error-message">
          <StatsPanel
            status={gauge.status}
            powerDraw={telemetry.powerDraw}
            energyAddedKwh={telemetry.energyAddedKwh}
            voltage={telemetry.voltage}
            current={telemetry.current}
            errorMessage={errorMessage}
            sessionStartTime={session.sessionStartTime}
            startPercent={gauge.startPercent ?? gauge.currentPercent}
            currentPercent={gauge.currentPercent}
            targetPercent={gauge.targetPercent}
            vehicle={selectedVehicle}
            carbonIntensity={carbonIntensity}
            renderTimeMs={session.renderTimeMs}
            costPerKwh={costPerKwh}
          />
        </div>
      </div>

      <div className="min-[1150px]:hidden">
        <StatsPanel
          status={gauge.status}
          powerDraw={telemetry.powerDraw}
          energyAddedKwh={telemetry.energyAddedKwh}
          voltage={telemetry.voltage}
          current={telemetry.current}
          errorMessage={errorMessage}
          sessionStartTime={session.sessionStartTime}
          startPercent={gauge.startPercent ?? gauge.currentPercent}
          currentPercent={gauge.currentPercent}
          targetPercent={gauge.targetPercent}
          vehicle={selectedVehicle}
          carbonIntensity={carbonIntensity}
          renderTimeMs={session.renderTimeMs}
          costPerKwh={costPerKwh}
        />
      </div>
    </div>
  );
}
