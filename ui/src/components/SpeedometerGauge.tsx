"use client";

import type { Schedule } from "@/lib/schemas";

import { GaugeFace } from "@/components/GaugeFace";
import { GaugeInfo } from "@/components/GaugeInfo";
import { GaugeNeedle } from "@/components/GaugeNeedle";
import { GaugeOverlay } from "@/components/GaugeOverlay";
import { GaugeScale } from "@/components/GaugeScale";
import { useGaugeDrag } from "@/components/useGaugeDrag";
import { KEYBOARD_COMMIT_DEBOUNCE_MS } from "@/lib/constants";
import { useGaugeStore } from "@/stores/gaugeStore";
import { snapPercentage } from "@/utils/snapping";
import React, { useRef, useCallback, useEffect, useState } from "react";

interface MaintenancePlugState {
  powerOn: boolean;
  online: boolean;
}

interface SpeedometerGaugeProps {
  startPercent: number;
  currentPercent: number;
  targetPercent: number;
  status:
    | "idle"
    | "charging"
    | "pending"
    | "conditioning"
    | "holding"
    | "error";
  onStartStop: () => void;
  isActionPending?: boolean;
  onDragStart?: () => void;
  onDragEnd?: (current: number, target: number) => void;
  tasmotaConnected?: boolean | null;
  schedule?: Schedule | null;
  onOpenSchedule?: () => void;
  maintenance?: MaintenancePlugState | null;
  onToggleMaintenance?: () => void;
  isMaintenancePending?: boolean;
}

const VIEW_BOX = 300;
// Crop 20 units off the bottom of the square viewBox. The gauge circle
// (center y=150, radius=126) extends to y=276. Setting height to 280 keeps
// the circle complete (4-unit margin) while removing the empty space below.
const VIEW_BOX_H = 280;

function SpeedometerGauge({
  startPercent,
  currentPercent: propsCurrentPercent,
  targetPercent: propsTargetPercent,
  status,
  onStartStop,
  isActionPending,
  onDragStart,
  onDragEnd,
  tasmotaConnected,
  schedule,
  onOpenSchedule,
  maintenance,
  onToggleMaintenance,
  isMaintenancePending,
}: SpeedometerGaugeProps) {
  // Subscribe to store percents for real-time drag feedback.
  const storeCurrent = useGaugeStore((s) => s.currentPercent);
  const storeTarget = useGaugeStore((s) => s.targetPercent);
  const isDragging = useGaugeStore((s) => s.isDragging);

  // Hold dragged percents in local state until props catch up from API response.
  // This prevents the gauge from reverting to stale props after a drag ends.
  const [draggedCurrent, setDraggedCurrent] = useState<number | null>(null);
  const [draggedTarget, setDraggedTarget] = useState<number | null>(null);

  // Clear local dragged values when props update to match (API confirmed).
  // Uses functional updates to avoid cascading renders.
  useEffect(() => {
    setDraggedCurrent((prev) =>
      prev !== null && propsCurrentPercent === prev ? null : prev,
    );
    setDraggedTarget((prev) =>
      prev !== null && propsTargetPercent === prev ? null : prev,
    );
  }, [propsCurrentPercent, propsTargetPercent]);

  // During active drag, render from store for real-time marker movement.
  // After drag ends, render from local state until props catch up.
  // Otherwise, render from props.
  const currentPercent =
    isDragging !== "none"
      ? storeCurrent
      : draggedCurrent !== null
        ? draggedCurrent
        : propsCurrentPercent;
  const targetPercent =
    isDragging !== "none"
      ? storeTarget
      : draggedTarget !== null
        ? draggedTarget
        : propsTargetPercent;

  const svgRef = useRef<SVGSVGElement>(null);

  // Sync store to props so hit detection in useGaugeDrag uses correct marker positions.
  // Only sync when not dragging to avoid interrupting active drag.
  // Skip in tests - tests manage store state directly via setPercents.
  useEffect(() => {
    if (typeof process !== "undefined" && process.env?.NODE_ENV === "test")
      return;
    const { isDragging, setPercents } = useGaugeStore.getState();
    if (isDragging === "none") {
      setPercents(propsCurrentPercent, propsTargetPercent);
    }
  }, [propsCurrentPercent, propsTargetPercent]);

  // Wrap onDragEnd to capture store values into local state for immediate UI feedback.
  const wrappedOnDragEnd = useCallback(
    (draggedCur: number, draggedTgt: number) => {
      setDraggedCurrent(draggedCur);
      setDraggedTarget(draggedTgt);
      onDragEnd?.(draggedCur, draggedTgt);
    },
    [onDragEnd],
  );

  const { draggingGauge, hoveringMarker, pointerProps } = useGaugeDrag({
    status,
    onDragStart,
    onDragEnd: wrappedOnDragEnd,
    svgRef,
  });

  // Keyboard edits bypass the pointer drag-end path, so debounce a commit of
  // the settled store values through the same callback to persist them.
  const keyboardCommitRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    return () => {
      if (keyboardCommitRef.current) clearTimeout(keyboardCommitRef.current);
    };
  }, []);
  const scheduleKeyboardCommit = useCallback(() => {
    if (keyboardCommitRef.current) clearTimeout(keyboardCommitRef.current);
    keyboardCommitRef.current = setTimeout(() => {
      const { currentPercent, targetPercent } = useGaugeStore.getState();
      wrappedOnDragEnd(currentPercent, targetPercent);
    }, KEYBOARD_COMMIT_DEBOUNCE_MS);
  }, [wrappedOnDragEnd]);

  const STEP_SIZE = 1;
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<SVGSVGElement>) => {
      const setPercents = useGaugeStore.getState().setPercents;
      const isArrowKey =
        e.key === "ArrowUp" ||
        e.key === "ArrowRight" ||
        e.key === "ArrowDown" ||
        e.key === "ArrowLeft";
      if (e.key === "ArrowUp" || e.key === "ArrowRight") {
        e.preventDefault();
        const field = e.shiftKey ? "current" : "target";
        if (field === "target") {
          const next = Math.min(100, snapPercentage(targetPercent + STEP_SIZE));
          setPercents(currentPercent, next);
        } else if (
          status !== "charging" &&
          status !== "conditioning" &&
          status !== "holding"
        ) {
          const next = Math.min(
            targetPercent,
            snapPercentage(currentPercent + STEP_SIZE),
          );
          setPercents(next, targetPercent);
        }
      } else if (e.key === "ArrowDown" || e.key === "ArrowLeft") {
        e.preventDefault();
        const field = e.shiftKey ? "current" : "target";
        if (field === "target") {
          const next = Math.max(0, snapPercentage(targetPercent - STEP_SIZE));
          if (
            status === "charging" ||
            status === "conditioning" ||
            status === "holding"
          ) {
            const eff = Math.max(next, currentPercent);
            setPercents(currentPercent, eff);
          } else {
            setPercents(currentPercent, next);
          }
        } else if (
          status !== "charging" &&
          status !== "conditioning" &&
          status !== "holding"
        ) {
          const next = Math.max(0, snapPercentage(currentPercent - STEP_SIZE));
          setPercents(next, targetPercent);
        }
      }
      if (isArrowKey) scheduleKeyboardCommit();
    },
    [currentPercent, targetPercent, status, scheduleKeyboardCommit],
  );

  return (
    <div className="flex flex-col items-center justify-center">
      <div
        data-testid="speedometer-gauge"
        className="relative w-full max-w-[640px] max-h-[640px] h-full mx-auto"
      >
        <div className="relative w-full h-full">
          <svg
            ref={svgRef}
            viewBox={`0 0 ${VIEW_BOX} ${VIEW_BOX_H}`}
            data-testid="speedometer-gauge-svg"
            className="w-full h-full outline-none"
            role="slider"
            tabIndex={0}
            aria-label="Speedometer gauge"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={targetPercent}
            aria-valuetext={`Current ${Math.floor(currentPercent)}%, target ${Math.floor(targetPercent)}%`}
            style={{
              userSelect: "none",
              WebkitUserSelect: "none",
              touchAction: "none",
              WebkitTapHighlightColor: "transparent",
              cursor: draggingGauge
                ? "grabbing"
                : hoveringMarker
                  ? "grab"
                  : "default",
            }}
            onKeyDown={handleKeyDown}
            {...pointerProps}
          >
            <foreignObject
              x="0"
              y="0"
              width={VIEW_BOX}
              height={VIEW_BOX}
              className="overflow-visible"
            >
              <div className="absolute inset-0">
                <GaugeFace
                  currentPercent={currentPercent}
                  startPercent={startPercent}
                  status={status}
                />
              </div>
              <div className="absolute inset-0">
                <GaugeScale
                  targetPercent={targetPercent}
                  status={status}
                  startPercent={startPercent}
                />
              </div>
              <div className="absolute inset-0">
                <GaugeInfo currentPercent={currentPercent} />
              </div>
              <div className="absolute inset-0">
                <GaugeNeedle currentPercent={currentPercent} />
              </div>
            </foreignObject>
          </svg>

          <GaugeOverlay
            status={status}
            currentPercent={currentPercent}
            targetPercent={targetPercent}
            onStartStop={onStartStop}
            isActionPending={isActionPending}
            tasmotaConnected={tasmotaConnected}
            schedule={schedule}
            onOpenSchedule={onOpenSchedule}
            maintenance={maintenance}
            onToggleMaintenance={onToggleMaintenance}
            isMaintenancePending={isMaintenancePending}
          />
        </div>
      </div>
    </div>
  );
}

export default SpeedometerGauge;
