"use client";

import type { SchedulePayload } from "@/hooks/useSchedule";
import type { Schedule } from "@/lib/schemas";

import Toggle from "@/components/Toggle";
import { useCallback, useId, useState, useEffect, useRef } from "react";

interface ScheduleFormProps {
  schedule: Schedule | null | undefined;
  onSave: (payload: SchedulePayload) => void;
  isSaving?: boolean;
  /** When true, shows a Skip button instead of a Cancel button (for wizard use). */
  onSkip?: () => void;
  saveLabel?: string;
}

export default function ScheduleForm({
  schedule,
  onSave,
  isSaving,
  onSkip,
  saveLabel = "Save",
}: ScheduleFormProps) {
  const dailyTimeId = useId();
  const windowStartId = useId();
  const windowEndId = useId();
  const readyById = useId();

  const [type, setType] = useState<"daily" | "carbon_aware">(
    () => (schedule?.type ?? "daily") as "daily" | "carbon_aware",
  );
  const [enabled, setEnabled] = useState(() => schedule?.enabled ?? false);
  const [dailyTime, setDailyTime] = useState(() => schedule?.time ?? "01:00");
  const [windowStart, setWindowStart] = useState(
    () => schedule?.windowStart ?? "01:00",
  );
  const [windowEnd, setWindowEnd] = useState(
    () => schedule?.windowEnd ?? "06:00",
  );
  const [twoStageEnabled, setTwoStageEnabled] = useState(
    () => schedule?.readyBy != null,
  );
  const [readyBy, setReadyBy] = useState(() => schedule?.readyBy ?? "07:00");
  const [carbonTwoStageEnabled, setCarbonTwoStageEnabled] = useState(
    () => schedule?.twoStage ?? false,
  );
  const [formError, setFormError] = useState<string | null>(null);

  const prevScheduleRef = useRef(schedule);
  useEffect(() => {
    if (schedule !== prevScheduleRef.current) {
      prevScheduleRef.current = schedule;
      setType((schedule?.type ?? "daily") as "daily" | "carbon_aware");
      setEnabled(schedule?.enabled ?? false);
      setDailyTime(schedule?.time ?? "01:00");
      setWindowStart(schedule?.windowStart ?? "01:00");
      setWindowEnd(schedule?.windowEnd ?? "06:00");
      setTwoStageEnabled(schedule?.readyBy != null);
      setReadyBy(schedule?.readyBy ?? "07:00");
      setCarbonTwoStageEnabled(schedule?.twoStage ?? false);
    }
  }, [schedule]);

  const handleSave = useCallback(() => {
    if (type === "carbon_aware") {
      if (windowStart === windowEnd) {
        setFormError("Start and ready-by times must differ.");
        return;
      }
      onSave({
        type: "carbon_aware",
        windowStart,
        windowEnd,
        twoStage: carbonTwoStageEnabled,
        enabled,
      });
    } else {
      if (twoStageEnabled && readyBy === dailyTime) {
        setFormError("Ready by must differ from start time.");
        return;
      }
      onSave({
        type: "daily",
        time: dailyTime,
        ...(twoStageEnabled ? { readyBy } : {}),
        enabled,
      });
    }
  }, [
    type,
    enabled,
    dailyTime,
    windowStart,
    windowEnd,
    twoStageEnabled,
    readyBy,
    carbonTwoStageEnabled,
    onSave,
  ]);

  return (
    <div className="space-y-5">
      {/* Enable toggle */}
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-fg">Enabled</p>
          <p className="text-xs text-fg-muted mt-0.5">
            Auto-start charging on this schedule
          </p>
        </div>
        <Toggle checked={enabled} onChange={setEnabled} label="Enabled" />
      </div>

      {/* Type switcher */}
      <div
        className={`space-y-4 transition-opacity duration-200 ${enabled ? "opacity-100" : "opacity-40 pointer-events-none"}`}
      >
        <div className="flex rounded-lg overflow-hidden border border-border">
          <button
            type="button"
            onClick={() => setType("daily")}
            className={`flex-1 flex items-center justify-center gap-2 py-2 text-sm font-medium transition-colors
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500
              ${type === "daily" ? "bg-blue-600 text-fg" : "bg-surface text-fg-muted hover:text-fg hover:bg-surface-hover"}`}
          >
            <i className="fa-regular fa-clock" aria-hidden="true" />
            Daily
          </button>
          <button
            type="button"
            onClick={() => setType("carbon_aware")}
            className={`flex-1 flex items-center justify-center gap-2 py-2 text-sm font-medium transition-colors
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500
              ${type === "carbon_aware" ? "bg-green-700 text-fg" : "bg-surface text-fg-muted hover:text-fg hover:bg-surface-hover"}`}
          >
            <i className="fa-solid fa-leaf" aria-hidden="true" />
            Carbon-aware
          </button>
        </div>

        {schedule?.targetUnreachable && (
          <div
            className="text-xs text-amber-700 bg-amber-50 border border-amber-200 dark:text-amber-300 dark:bg-amber-950/40 dark:border-amber-800/50 rounded-md px-3 py-2"
            role="alert"
            data-testid="target-unreachable-warning"
          >
            This schedule doesn&apos;t leave enough time to reach your target by
            the ready-by time. Widen the window, choose an earlier ready-by, or
            lower the target.
          </div>
        )}

        {type === "daily" ? (
          <div className="space-y-3">
            <p className="text-xs text-fg-muted">
              Start charging at a fixed time each day if below target.
            </p>
            <div className="flex items-center gap-3">
              <label
                htmlFor={dailyTimeId}
                className="text-xs font-medium text-fg-muted uppercase tracking-wider whitespace-nowrap"
              >
                Start time
              </label>
              <input
                id={dailyTimeId}
                type="time"
                value={dailyTime}
                onChange={(e) => {
                  setDailyTime(e.target.value);
                  setFormError(null);
                }}
                className="bg-surface text-fg px-3 py-1.5 rounded-md text-sm
                  border border-border focus:border-blue-500 focus:outline-none
                  focus:ring-1 focus:ring-blue-500"
              />
            </div>

            <div className="flex items-center justify-between pt-1">
              <div>
                <p className="text-xs font-medium text-fg-secondary">
                  Two-stage charging
                </p>
                <p className="text-xs text-fg-muted mt-0.5">
                  Charge to 80% of your target now, hold, then finish to 100% of
                  your target by the ready-by time.
                </p>
              </div>
              <Toggle
                checked={twoStageEnabled}
                onChange={setTwoStageEnabled}
                label="Two-stage charging"
              />
            </div>
            {twoStageEnabled && (
              <div className="flex items-center gap-3">
                <label
                  htmlFor={readyById}
                  className="text-xs font-medium text-fg-muted uppercase tracking-wider whitespace-nowrap"
                >
                  Ready by
                </label>
                <input
                  id={readyById}
                  type="time"
                  value={readyBy}
                  onChange={(e) => {
                    setReadyBy(e.target.value);
                    setFormError(null);
                  }}
                  className="bg-surface text-fg px-3 py-1.5 rounded-md text-sm
                    border border-border focus:border-blue-500 focus:outline-none
                    focus:ring-1 focus:ring-blue-500"
                />
              </div>
            )}
            {twoStageEnabled && schedule?.estimatedPlan && (
              <div
                className="text-xs text-fg-muted bg-surface/60 rounded-md px-3 py-2 space-y-1"
                data-testid="estimated-plan"
              >
                <p className="text-fg-secondary font-medium">Estimated plan</p>
                <p>
                  Stage 1: {schedule.estimatedPlan.stage1Start} –{" "}
                  {schedule.estimatedPlan.stage1End} (to 80%)
                </p>
                <p>Hold until {schedule.estimatedPlan.stage2Start}</p>
                <p>
                  Stage 2: {schedule.estimatedPlan.stage2Start} –{" "}
                  {schedule.estimatedPlan.stage2End} (to 100%)
                </p>
              </div>
            )}
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-xs text-fg-muted">
              Shifts charging to the cleanest hours of your window. Guarantees
              the vehicle reaches target by the ready-by time.
            </p>
            <div className="space-y-2">
              <div className="flex items-center gap-3">
                <label
                  htmlFor={windowStartId}
                  className="text-xs font-medium text-fg-muted uppercase tracking-wider w-20 shrink-0"
                >
                  Earliest
                </label>
                <input
                  id={windowStartId}
                  type="time"
                  value={windowStart}
                  onChange={(e) => {
                    setWindowStart(e.target.value);
                    setFormError(null);
                  }}
                  className="bg-surface text-fg px-3 py-1.5 rounded-md text-sm
                    border border-border focus:border-blue-500 focus:outline-none
                    focus:ring-1 focus:ring-blue-500"
                />
              </div>
              <div className="flex items-center gap-3">
                <label
                  htmlFor={windowEndId}
                  className="text-xs font-medium text-fg-muted uppercase tracking-wider w-20 shrink-0"
                >
                  Ready by
                </label>
                <input
                  id={windowEndId}
                  type="time"
                  value={windowEnd}
                  onChange={(e) => {
                    setWindowEnd(e.target.value);
                    setFormError(null);
                  }}
                  className="bg-surface text-fg px-3 py-1.5 rounded-md text-sm
                    border border-border focus:border-blue-500 focus:outline-none
                    focus:ring-1 focus:ring-blue-500"
                />
              </div>
            </div>

            <div className="flex items-center justify-between pt-1">
              <div>
                <p className="text-xs font-medium text-fg-secondary">
                  Two-stage charging
                </p>
                <p className="text-xs text-fg-muted mt-0.5">
                  Charge to 80% of your target in the cleanest early slot, hold,
                  then finish to 100% of your target before the ready-by time —
                  balancing low carbon with less time spent at a high charge.
                </p>
              </div>
              <Toggle
                checked={carbonTwoStageEnabled}
                onChange={setCarbonTwoStageEnabled}
                label="Carbon-aware two-stage charging"
              />
            </div>
            {carbonTwoStageEnabled && schedule?.estimatedPlan && (
              <div
                className="text-xs text-fg-muted bg-surface/60 rounded-md px-3 py-2 space-y-1"
                data-testid="estimated-plan"
              >
                <p className="text-fg-secondary font-medium">Estimated plan</p>
                <p>
                  Stage 1: {schedule.estimatedPlan.stage1Start} –{" "}
                  {schedule.estimatedPlan.stage1End} (to 80%)
                </p>
                <p>Hold until {schedule.estimatedPlan.stage2Start}</p>
                <p>
                  Stage 2: {schedule.estimatedPlan.stage2Start} –{" "}
                  {schedule.estimatedPlan.stage2End} (to 100%)
                </p>
              </div>
            )}
          </div>
        )}
        {formError && (
          <p className="text-xs text-danger" role="alert">
            {formError}
          </p>
        )}
      </div>

      {/* Actions */}
      <div className="flex gap-2">
        {onSkip && (
          <button
            type="button"
            onClick={onSkip}
            className="flex-1 px-4 py-2 text-sm font-medium text-fg-secondary hover:text-fg rounded-lg
              hover:bg-surface-hover transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          >
            Skip
          </button>
        )}
        <button
          type="button"
          onClick={handleSave}
          disabled={isSaving}
          className="flex-1 px-4 py-2 text-sm font-medium text-fg rounded-lg bg-blue-600
            hover:bg-blue-500 active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed
            transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        >
          {isSaving ? "Saving…" : saveLabel}
        </button>
      </div>
    </div>
  );
}
