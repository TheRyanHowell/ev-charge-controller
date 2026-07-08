"use client";

import type {
  Plug,
  Vehicle,
  CarbonIntensity,
  InitialChargeSession,
  ProvisioningResult,
  Schedule,
} from "@/lib/schemas";
import type { PowerReading, SOCSnapshot } from "@/types/chart";

import ChargeControl from "@/app/features/ChargeControl";
import ChartsSection from "@/app/features/ChartsSection";
import DashboardHeader from "@/app/features/DashboardHeader";
import StatusBar from "@/app/features/StatusBar";
import AddPlugModal from "@/components/AddPlugModal";
import ErrorBoundary from "@/components/ErrorBoundary";
import FirstRunWizard from "@/components/FirstRunWizard";
import ScheduleModal from "@/components/ScheduleModal";
import SettingsModal from "@/components/SettingsModal";
import VehicleChips from "@/components/VehicleChips";
import {
  useVehicle,
  useChargeSession,
  useSchedule,
  useCarbonIntensity,
} from "@/hooks";
import { usePlug } from "@/hooks/usePlug";
import { useTariff } from "@/hooks/useTariff";
import { apiGet } from "@/lib/api";
import { ensurePushSubscription } from "@/lib/push";
import { queryKeys } from "@/lib/queryKeys";
import { PlugSchema, VehicleSchema } from "@/lib/schemas";
import { useGaugeStore } from "@/stores/gaugeStore";
import { activeRatePence } from "@/utils/gauge";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useCallback, useState } from "react";

interface DashboardProps {
  initialVehicles?: Vehicle[];
  initialPlugs?: Plug[];
  initialSession?: InitialChargeSession | null;
  initialCarbonIntensity?: CarbonIntensity | null;
  initialPowerReadings?: PowerReading[];
  initialSocSnapshots?: SOCSnapshot[];
  initialSchedule?: Schedule | null;
  initialSelectedVehicleId?: string | null;
  renderTimeMs?: number;
}

export default function Dashboard({
  initialVehicles,
  initialPlugs,
  initialSession,
  initialCarbonIntensity,
  initialPowerReadings,
  initialSocSnapshots,
  initialSchedule,
  initialSelectedVehicleId,
  renderTimeMs,
}: DashboardProps) {
  const storeInitialized = useGaugeStore((s) => s.initialized);
  const queryClient = useQueryClient();
  const [wizardCompleted, setWizardCompleted] = useState(false);
  const [addVehicleOpen, setAddVehicleOpen] = useState(false);
  const [add12VOpen, setAdd12VOpen] = useState(false);
  const [scheduleOpen, setScheduleOpen] = useState(false);

  const {
    plugs,
    selectedVehicleId,
    selectVehicle,
    updatePlug,
    deletePlug,
    toggleMaintenancePower,
    isTogglingPower,
    isLoading: plugsLoading,
  } = usePlug(initialPlugs, renderTimeMs, initialSelectedVehicleId);

  const showWizard = !plugsLoading && plugs.length === 0 && !wizardCompleted;

  const {
    vehicles,
    isLoading: vehiclesLoading,
    handleOpenSettings,
    isSettingsOpen,
    closeSettings,
    updatePercents,
    updateNotificationPrefs,
    tempError,
  } = useVehicle({
    initialVehicles,
    initialDataUpdatedAt: renderTimeMs,
  });

  // Vehicle-centric: derive plugs from the selected vehicle.
  const selectedVehicle = useMemo(
    () => vehicles.find((v) => v.id === selectedVehicleId) ?? null,
    [vehicles, selectedVehicleId],
  );

  const chargingPlug = useMemo(
    () =>
      plugs.find(
        (p) => p.type === "charging" && p.vehicleId === selectedVehicleId,
      ) ?? null,
    [plugs, selectedVehicleId],
  );

  const maintenancePlug = useMemo(
    () =>
      plugs.find(
        (p) => p.type === "maintenance" && p.vehicleId === selectedVehicleId,
      ) ?? null,
    [plugs, selectedVehicleId],
  );

  const storeCurrentPercent = useGaugeStore((s) => s.currentPercent);
  const storeTargetPercent = useGaugeStore((s) => s.targetPercent);
  const currentPercent = storeInitialized
    ? storeCurrentPercent
    : (initialSession?.currentPercent ?? selectedVehicle?.currentPercent ?? 20);
  const targetPercent = storeInitialized
    ? storeTargetPercent
    : (initialSession?.targetPercent ?? selectedVehicle?.targetPercent ?? 80);

  const {
    session,
    chargeStartPercent,
    errorMessage,
    startCharging,
    stopCharging,
    isChargingActionPending,
    isStopActionPending,
    handleTargetChargeUpdate,
    onDragStart,
    onDragEnd: chargeOnDragEnd,
    clearError,
    sessionStartTime,
  } = useChargeSession(
    selectedVehicle,
    chargingPlug?.id ?? null,
    initialSession,
  );

  const { schedule, saveSchedule } = useSchedule(
    chargingPlug?.id ?? null,
    // Only use SSR schedule when the same vehicle is selected as was on the server.
    selectedVehicleId === initialSelectedVehicleId
      ? initialSchedule
      : undefined,
    renderTimeMs,
  );

  const { carbonIntensity } = useCarbonIntensity(initialCarbonIntensity);

  const { settings: tariff } = useTariff();
  const costPerKwh = useMemo(
    () => activeRatePence(tariff, new Date()),
    [tariff],
  );

  useEffect(() => {
    ensurePushSubscription();
  }, []);

  const gaugeReset = useGaugeStore((s) => s.reset);
  const markInitialized = useGaugeStore((s) => s.markInitialized);
  const setPercents = useGaugeStore((s) => s.setPercents);
  useEffect(() => {
    gaugeReset();
  }, [gaugeReset]);

  // Re-sync gauge when vehicle selection changes.
  useEffect(() => {
    if (!selectedVehicle) return;
    if (useGaugeStore.getState().isDragging !== "none") return;
    const c = selectedVehicle.currentPercent ?? 0;
    const t = selectedVehicle.targetPercent ?? 80;
    setPercents(c, t);
    markInitialized();
  }, [selectedVehicle, selectedVehicleId, setPercents, markInitialized]);

  const isChargingOrPending =
    session.status === "charging" ||
    session.status === "pending" ||
    session.status === "conditioning" ||
    session.status === "holding";
  const isActive =
    session.status === "charging" ||
    session.status === "conditioning" ||
    session.status === "holding";
  const powerDraw = isActive ? session.powerDraw : 0;
  const energyAddedKwh = isActive ? session.energyAddedKwh : null;
  const voltage = isActive ? session.voltage : null;
  const current = isActive ? session.current : null;

  const isLoading = vehiclesLoading && !initialVehicles;

  const handleUpdatePlugName = useCallback(
    (name: string) => {
      if (chargingPlug) void updatePlug({ plugId: chargingPlug.id, name });
    },
    [chargingPlug, updatePlug],
  );

  const handleDeletePlug = useCallback(() => {
    if (chargingPlug) void deletePlug(chargingPlug.id);
  }, [chargingPlug, deletePlug]);

  const handleUpdateMaintenanceName = useCallback(
    (name: string) => {
      if (maintenancePlug)
        void updatePlug({ plugId: maintenancePlug.id, name });
    },
    [maintenancePlug, updatePlug],
  );

  const handleDeleteMaintenance = useCallback(() => {
    if (maintenancePlug) void deletePlug(maintenancePlug.id);
  }, [maintenancePlug, deletePlug]);

  const handlePlugCreated = useCallback(
    (result: ProvisioningResult) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.plugs.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.vehicles.all });
      if (result.plug.vehicleId) {
        selectVehicle(result.plug.vehicleId);
      }
    },
    [selectVehicle, queryClient],
  );

  const handleWizardComplete = useCallback(async () => {
    try {
      const [freshPlugs, freshVehicles] = await Promise.all([
        apiGet("/api/plugs", PlugSchema),
        apiGet("/api/vehicles", VehicleSchema),
      ]);
      queryClient.setQueryData(queryKeys.plugs.all, freshPlugs);
      queryClient.setQueryData(queryKeys.vehicles.all, freshVehicles);
      // Select the vehicle of the first plug.
      const firstPlug = freshPlugs[0];
      if (firstPlug?.vehicleId) {
        selectVehicle(firstPlug.vehicleId);
      }
      setWizardCompleted(true);
    } catch (e) {
      console.error("[Dashboard] handleWizardComplete - fetch failed:", e);
    }
  }, [queryClient, selectVehicle]);

  // Threads the current plug through to updatePercents so its onSettled can
  // refresh the carbon-aware schedule's forecast-based start estimate - a new
  // target percent changes how long charging will take.
  const updatePercentsForCurrentPlug = useCallback(
    (vehicleId: string, current: number, target: number) =>
      updatePercents(vehicleId, current, target, chargingPlug?.id ?? null),
    [updatePercents, chargingPlug],
  );

  const handlers = useMemo(
    () => ({
      onStartCharging: startCharging,
      onStopCharging: stopCharging,
      onDragStart,
      onChargeDragEnd: chargeOnDragEnd,
      clearError,
      handleTargetChargeUpdate,
      updatePercents: updatePercentsForCurrentPlug,
    }),
    [
      startCharging,
      stopCharging,
      onDragStart,
      chargeOnDragEnd,
      clearError,
      handleTargetChargeUpdate,
      updatePercentsForCurrentPlug,
    ],
  );

  if (isLoading) {
    return (
      <main className="min-h-screen bg-page-bg text-white flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white mx-auto mb-4"></div>
          <p className="text-gray-400">Loading...</p>
        </div>
      </main>
    );
  }

  if (showWizard) {
    return <FirstRunWizard onComplete={handleWizardComplete} />;
  }

  return (
    <ErrorBoundary>
      <main className="min-h-screen bg-page-bg text-white">
        <div className="w-full max-w-6xl mx-auto px-4 py-6 sm:px-6 sm:py-8">
          <DashboardHeader onOpenSettings={handleOpenSettings} />

          <VehicleChips
            vehicles={vehicles}
            plugs={plugs}
            selectedVehicleId={selectedVehicleId}
            onSelect={selectVehicle}
          />

          <StatusBar tempError={tempError} selectedVehicle={selectedVehicle} />

          <ChargeControl
            selectedVehicle={selectedVehicle}
            isActive={isActive}
            schedule={schedule}
            onOpenSchedule={() => setScheduleOpen(true)}
            gauge={{
              currentPercent,
              targetPercent,
              startPercent: chargeStartPercent,
              status: session.status as
                | "idle"
                | "charging"
                | "pending"
                | "conditioning"
                | "holding"
                | "error",
              estimatedResumeTime:
                session.status === "holding"
                  ? session.estimatedResumeTime
                  : null,
            }}
            session={{
              isChargingOrPending,
              isActionPending: isChargingActionPending || isStopActionPending,
              sessionStartTime,
              renderTimeMs: initialSession?.renderTimeMs ?? renderTimeMs,
            }}
            telemetry={{ powerDraw, energyAddedKwh, voltage, current }}
            errorMessage={errorMessage}
            tasmotaConnected={chargingPlug?.online ?? null}
            carbonIntensity={carbonIntensity}
            costPerKwh={costPerKwh}
            handlers={handlers}
            maintenancePlug={maintenancePlug}
            onToggleMaintenance={() => {
              if (maintenancePlug) {
                void toggleMaintenancePower({
                  plugId: maintenancePlug.id,
                  on: !maintenancePlug.powerOn,
                });
              }
            }}
            isMaintenancePending={isTogglingPower}
          />

          <ChartsSection
            vehicleId={selectedVehicle?.id ?? null}
            shouldPoll={!!isChargingOrPending}
            initialPowerReadings={initialPowerReadings}
            initialSocSnapshots={initialSocSnapshots}
          />
        </div>

        <ErrorBoundary fallback={null}>
          <SettingsModal
            isOpen={isSettingsOpen}
            onClose={closeSettings}
            plug={chargingPlug}
            vehicles={vehicles}
            onUpdateName={handleUpdatePlugName}
            onDelete={handleDeletePlug}
            onUpdateNotificationPrefs={updateNotificationPrefs}
            maintenancePlug={maintenancePlug}
            onAdd12V={() => setAdd12VOpen(true)}
            onAddChargingPlug={() => setAddVehicleOpen(true)}
            onDeleteMaintenance={handleDeleteMaintenance}
            onUpdateMaintenanceName={handleUpdateMaintenanceName}
          />
          <ScheduleModal
            isOpen={scheduleOpen}
            onClose={() => setScheduleOpen(false)}
            schedule={schedule}
            onSave={(payload) => {
              void saveSchedule(payload);
              setScheduleOpen(false);
            }}
            isSaving={false}
          />
        </ErrorBoundary>

        <AddPlugModal
          isOpen={addVehicleOpen}
          onClose={() => setAddVehicleOpen(false)}
          onPlugCreated={handlePlugCreated}
        />
        <AddPlugModal
          isOpen={add12VOpen}
          onClose={() => setAdd12VOpen(false)}
          onPlugCreated={handlePlugCreated}
          existingVehicleId={selectedVehicleId}
        />
      </main>
    </ErrorBoundary>
  );
}
