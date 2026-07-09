"use client";

import type { Schedule } from "@/lib/schemas";

interface MaintenancePlugState {
  powerOn: boolean;
  online: boolean;
}

interface GaugeOverlayProps {
  status:
    | "idle"
    | "charging"
    | "pending"
    | "conditioning"
    | "holding"
    | "error";
  currentPercent: number;
  targetPercent: number;
  onStartStop: () => void;
  isActionPending?: boolean;
  tasmotaConnected?: boolean | null;
  /** HH:MM best guess for when a holding session will resume; only shown while status is "holding". */
  estimatedResumeTime?: string | null;
  schedule?: Schedule | null;
  onOpenSchedule?: () => void;
  maintenance?: MaintenancePlugState | null;
  onToggleMaintenance?: () => void;
  isMaintenancePending?: boolean;
}

function scheduleIcon(type: Schedule["type"] | undefined): string {
  return type === "carbon_aware" ? "fa-solid fa-leaf" : "fa-regular fa-clock";
}

function scheduleLabel(schedule: Schedule | null | undefined): string {
  if (!schedule) return "";
  if (schedule.type === "carbon_aware") {
    return schedule.estimatedStartTime ?? schedule.windowEnd ?? "";
  }
  return schedule.time;
}

// scheduleLabelWord describes what scheduleTime represents: once a forecast-based
// start estimate exists we're showing when charging begins, not the ready-by deadline.
function scheduleLabelWord(schedule: Schedule | null | undefined): string {
  if (schedule?.type === "carbon_aware" && !schedule.estimatedStartTime) {
    return "ready by";
  }
  return "starts at";
}

export function GaugeOverlay({
  status,
  currentPercent,
  targetPercent,
  tasmotaConnected,
  onStartStop,
  isActionPending,
  schedule,
  onOpenSchedule,
  maintenance,
  onToggleMaintenance,
  isMaintenancePending,
  estimatedResumeTime,
}: GaugeOverlayProps) {
  const isCharged = currentPercent >= targetPercent;

  const isChargingOrPending =
    status === "charging" ||
    status === "pending" ||
    status === "conditioning" ||
    status === "holding";
  const isDisabled =
    isActionPending ||
    (status !== "charging" &&
      status !== "pending" &&
      status !== "conditioning" &&
      status !== "holding" &&
      (isCharged || tasmotaConnected === false));

  const scheduleActive = schedule?.enabled ?? false;
  const scheduleTime = scheduleLabel(schedule);
  const scheduleUnreachable =
    scheduleActive && (schedule?.targetUnreachable ?? false);

  return (
    <>
      {/* Percentage + status text: fixed in upper half of gauge, no JS scaling needed */}
      <div className="absolute inset-0" style={{ pointerEvents: "none" }}>
        <div className="absolute inset-x-0 top-0 bottom-1/2 flex flex-col items-center justify-end pb-10">
          <div
            data-testid="gauge-percent"
            className="text-5xl font-semibold text-fg tabular-nums tracking-tight"
          >
            {currentPercent.toFixed(0)}%
          </div>
          <div className="text-xs text-fg-muted mt-1.5 uppercase tracking-[0.25em] font-medium">
            {status === "charging" && "Charging"}
            {status === "conditioning" && "Conditioning"}
            {status === "holding" && "Holding"}
            {status === "pending" && "Pending"}
            {status === "idle" &&
              !isCharged &&
              tasmotaConnected === false &&
              "Disconnected"}
            {status === "idle" &&
              !isCharged &&
              tasmotaConnected !== false &&
              "Ready"}
            {status === "error" && "Error"}
          </div>
          {status === "holding" && estimatedResumeTime && (
            <div
              className="text-[10px] text-fg-muted mt-1 normal-case tracking-normal font-normal"
              data-testid="estimated-resume-time"
            >
              resumes ~{estimatedResumeTime}
            </div>
          )}
        </div>
      </div>

      {/* START/STOP button: % of container so SSR HTML is correct without JS.
          top = 310/420 ≈ 73.81%, size = 100/420 ≈ 23.81% */}
      <div
        className="absolute left-1/2 -translate-x-1/2 -translate-y-1/2"
        style={{ top: "73.81%", width: "23.81%", aspectRatio: "1" }}
      >
        <button
          onClick={onStartStop}
          disabled={isDisabled}
          style={{ pointerEvents: "auto", width: "100%", height: "100%" }}
          className={`
            rounded-full text-fg font-bold tracking-wider text-lg
            transition-all duration-200
            focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-page-bg
            ${
              isChargingOrPending
                ? "bg-red-600 hover:bg-red-500 active:scale-[0.95] shadow-lg shadow-red-600/30"
                : isCharged || tasmotaConnected === false
                  ? "bg-surface-hover cursor-not-allowed text-fg-muted"
                  : "bg-green-600 hover:bg-green-500 active:scale-[0.95] shadow-lg shadow-green-600/30"
            }
          `}
          aria-label={
            isChargingOrPending
              ? "Stop charging"
              : isCharged
                ? "Charged"
                : "Start charging"
          }
        >
          <span className="font-extrabold tracking-widest">
            {isChargingOrPending ? "STOP" : isCharged ? "DONE" : "START"}
          </span>
        </button>
      </div>

      {/* 12V maintenance plug circle: top-right corner, only when a maintenance plug exists */}
      {maintenance && (
        <div
          className="absolute"
          style={{ top: "5%", right: "5%", width: "14%", aspectRatio: "1" }}
        >
          <button
            type="button"
            role="switch"
            aria-checked={maintenance.powerOn}
            aria-label={
              !maintenance.online
                ? "12V charger offline"
                : maintenance.powerOn
                  ? "12V charger on - tap to turn off"
                  : "12V charger off - tap to turn on"
            }
            onClick={onToggleMaintenance}
            disabled={isMaintenancePending}
            style={{ pointerEvents: "auto", width: "100%", height: "100%" }}
            data-testid="maintenance-circle"
            className={`
              rounded-full flex flex-col items-center justify-center
              border-2 transition-all duration-200
              focus-visible:ring-2 focus-visible:ring-cyan-400 focus-visible:ring-offset-1 focus-visible:ring-offset-transparent
              disabled:opacity-60 disabled:cursor-wait
              ${
                !maintenance.online
                  ? "bg-surface-raised/70 border-amber-500 text-warning"
                  : maintenance.powerOn
                    ? "bg-cyan-100 dark:bg-cyan-900/80 border-cyan-400 text-cyan-700 dark:text-cyan-300 hover:bg-cyan-200 dark:hover:bg-cyan-800/80 shadow-md shadow-cyan-500/20"
                    : "bg-surface-raised/70 border-border/50 text-fg-muted hover:text-fg-secondary hover:border-fg-muted"
              }
            `}
          >
            <span className="text-[13px] font-bold leading-none tracking-tight">
              12V
            </span>
          </button>
        </div>
      )}

      {/* Schedule button: anchored to the top-left corner of the container.
          Mirrors the 12V circle at top-right; both sit just outside the gauge ring. */}
      <div
        className="absolute"
        style={{ top: "5%", left: "5%", width: "14%", aspectRatio: "1" }}
      >
        <button
          type="button"
          onClick={onOpenSchedule}
          style={{ pointerEvents: "auto", width: "100%", height: "100%" }}
          className={`
            rounded-full flex flex-col items-center justify-center gap-1
            border-2 transition-all duration-200
            focus-visible:ring-2 focus-visible:ring-blue-400 focus-visible:ring-offset-1 focus-visible:ring-offset-transparent
            ${
              scheduleUnreachable
                ? "bg-amber-100 dark:bg-amber-950/70 border-amber-500 text-warning hover:bg-amber-200 dark:hover:bg-amber-900/70"
                : scheduleActive
                  ? schedule?.type === "carbon_aware"
                    ? "bg-green-100 dark:bg-green-900/80 border-green-400 text-green-700 dark:text-green-300 hover:bg-green-200 dark:hover:bg-green-800/80 shadow-md shadow-green-500/20"
                    : "bg-blue-100 dark:bg-blue-900/80 border-blue-400 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-800/80 shadow-md shadow-blue-500/20"
                  : "bg-surface-raised/70 border-border/50 text-fg-muted hover:text-fg-secondary hover:border-fg-muted"
            }
          `}
          aria-label={
            scheduleUnreachable
              ? `Schedule active but may not reach target - ${scheduleLabelWord(schedule)} ${scheduleTime}`
              : scheduleActive
                ? `Schedule active - ${scheduleLabelWord(schedule)} ${scheduleTime}`
                : schedule
                  ? `Schedule configured but disabled - ${scheduleTime}`
                  : "Configure charge schedule"
          }
          data-testid="schedule-circle"
        >
          <i
            className={`${scheduleUnreachable ? "fa-solid fa-triangle-exclamation" : scheduleIcon(schedule?.type)} text-lg leading-none`}
            aria-hidden="true"
          />
          {scheduleActive && (
            <span className="text-[10px] leading-none uppercase tracking-wide opacity-80">
              {scheduleUnreachable
                ? "Warning"
                : schedule?.type === "carbon_aware"
                  ? "Carbon"
                  : "Daily"}
            </span>
          )}
          {scheduleActive && scheduleTime ? (
            <span className="text-[12px] leading-none font-medium tabular-nums">
              {scheduleTime}
            </span>
          ) : null}
        </button>
      </div>
    </>
  );
}
