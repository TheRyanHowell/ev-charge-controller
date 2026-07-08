"use client";

import type { SchedulePayload } from "@/hooks/useSchedule";
import type { Schedule } from "@/lib/schemas";

import { useCallback, useId, useState, useEffect, useRef } from "react";

interface ScheduleFormProps {
  schedule: Schedule | null | undefined;
  onSave: (payload: SchedulePayload) => void;
  isSaving?: boolean;
  /** When true, shows a Skip button instead of a Cancel button (for wizard use). */
  onSkip?: () => void;
  saveLabel?: string;
}

function Toggle({
  checked,
  onChange,
  disabled,
  label,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-0
        transition-colors duration-200 ease-in-out focus-visible:outline-none
        focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2
        focus-visible:ring-offset-gray-900
        ${checked ? "bg-blue-500" : "bg-gray-600"}
        ${disabled ? "opacity-50 cursor-not-allowed" : ""}`}
    >
      <span
        className={`pointer-events-none relative inline-block h-5 w-5 rounded-full
          bg-white shadow ring-0 transition duration-200 ease-in-out
          ${checked ? "translate-x-5" : "translate-x-0"}`}
      />
    </button>
  );
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
          <p className="text-sm font-medium text-white">Enabled</p>
          <p className="text-xs text-gray-400 mt-0.5">
            Auto-start charging on this schedule
          </p>
        </div>
        <Toggle checked={enabled} onChange={setEnabled} label="Enabled" />
      </div>

      {/* Type switcher */}
      <div
        className={`space-y-4 transition-opacity duration-200 ${enabled ? "opacity-100" : "opacity-40 pointer-events-none"}`}
      >
        <div className="flex rounded-lg overflow-hidden border border-gray-700">
          <button
            type="button"
            onClick={() => setType("daily")}
            className={`flex-1 flex items-center justify-center gap-2 py-2 text-sm font-medium transition-colors
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500
              ${type === "daily" ? "bg-blue-600 text-white" : "bg-gray-800 text-gray-400 hover:text-white hover:bg-gray-700"}`}
          >
            <i className="fa-regular fa-clock" aria-hidden="true" />
            Daily
          </button>
          <button
            type="button"
            onClick={() => setType("carbon_aware")}
            className={`flex-1 flex items-center justify-center gap-2 py-2 text-sm font-medium transition-colors
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500
              ${type === "carbon_aware" ? "bg-green-700 text-white" : "bg-gray-800 text-gray-400 hover:text-white hover:bg-gray-700"}`}
          >
            <i className="fa-solid fa-leaf" aria-hidden="true" />
            Carbon-aware
          </button>
        </div>

        {type === "daily" ? (
          <div className="space-y-3">
            <p className="text-xs text-gray-400">
              Start charging at a fixed time each day if below target.
            </p>
            <div className="flex items-center gap-3">
              <label
                htmlFor={dailyTimeId}
                className="text-xs font-medium text-gray-400 uppercase tracking-wider whitespace-nowrap"
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
                className="bg-gray-800 text-white px-3 py-1.5 rounded-md text-sm
                  border border-gray-600 focus:border-blue-500 focus:outline-none
                  focus:ring-1 focus:ring-blue-500"
              />
            </div>

            <div className="flex items-center justify-between pt-1">
              <div>
                <p className="text-xs font-medium text-gray-300">
                  Two-stage charging
                </p>
                <p className="text-xs text-gray-500 mt-0.5">
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
                  className="text-xs font-medium text-gray-400 uppercase tracking-wider whitespace-nowrap"
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
                  className="bg-gray-800 text-white px-3 py-1.5 rounded-md text-sm
                    border border-gray-600 focus:border-blue-500 focus:outline-none
                    focus:ring-1 focus:ring-blue-500"
                />
              </div>
            )}
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-xs text-gray-400">
              Shifts charging to the cleanest hours of your window. Guarantees
              the vehicle reaches target by the ready-by time.
            </p>
            <div className="space-y-2">
              <div className="flex items-center gap-3">
                <label
                  htmlFor={windowStartId}
                  className="text-xs font-medium text-gray-400 uppercase tracking-wider w-20 shrink-0"
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
                  className="bg-gray-800 text-white px-3 py-1.5 rounded-md text-sm
                    border border-gray-600 focus:border-blue-500 focus:outline-none
                    focus:ring-1 focus:ring-blue-500"
                />
              </div>
              <div className="flex items-center gap-3">
                <label
                  htmlFor={windowEndId}
                  className="text-xs font-medium text-gray-400 uppercase tracking-wider w-20 shrink-0"
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
                  className="bg-gray-800 text-white px-3 py-1.5 rounded-md text-sm
                    border border-gray-600 focus:border-blue-500 focus:outline-none
                    focus:ring-1 focus:ring-blue-500"
                />
              </div>
            </div>

            <div className="flex items-center justify-between pt-1">
              <div>
                <p className="text-xs font-medium text-gray-300">
                  Two-stage charging
                </p>
                <p className="text-xs text-gray-500 mt-0.5">
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
                className="text-xs text-gray-400 bg-gray-800/60 rounded-md px-3 py-2 space-y-1"
                data-testid="estimated-plan"
              >
                <p className="text-gray-300 font-medium">Estimated plan</p>
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
          <p className="text-xs text-red-400" role="alert">
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
            className="flex-1 px-4 py-2 text-sm font-medium text-gray-300 hover:text-white rounded-lg
              hover:bg-gray-700 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          >
            Skip
          </button>
        )}
        <button
          type="button"
          onClick={handleSave}
          disabled={isSaving}
          className="flex-1 px-4 py-2 text-sm font-medium text-white rounded-lg bg-blue-600
            hover:bg-blue-500 active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed
            transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        >
          {isSaving ? "Saving…" : saveLabel}
        </button>
      </div>
    </div>
  );
}
