"use client";

import {
  GAUGE_ARC_START_ANGLE_DEG,
  GAUGE_ARC_SPAN_DEG,
  GAUGE_VIEWBOX_W,
  HIT_TOLERANCE_DEG,
  RADIANS_PER_DEGREE,
} from "@/lib/constants";
import { useGaugeStore } from "@/stores/gaugeStore";
import {
  gaugeAngleFromOffset,
  percentageToAngle,
  isAngleInGap,
  gaugeAngleToPercentage,
  angularDistance,
} from "@/utils/gauge";
import { snapToPercent } from "@/utils/snapping";
import { useState, useEffect, useCallback, useRef } from "react";

const START_ANGLE_RAD = GAUGE_ARC_START_ANGLE_DEG * RADIANS_PER_DEGREE;
const TOTAL_ARC_RAD = GAUGE_ARC_SPAN_DEG * RADIANS_PER_DEGREE;

// The gauge circle center is at (GAUGE_VIEWBOX_W/2, GAUGE_VIEWBOX_W/2) in the outer
// SVG's viewBox, but the outer viewBox height is 280 (not 300), so the center is at
// 53.6% of the rendered height - not 50%. getScreenCTM() transforms this viewBox
// point to the exact screen position, fixing the asymmetric grab radius. Falls back
// to the old rect-based path in environments where getScreenCTM is unavailable (jsdom).
function getAngleFromPointer(
  e: { clientX: number; clientY: number },
  svg: SVGSVGElement,
): number {
  try {
    const ctm = svg.getScreenCTM();
    if (ctm) {
      const pt = svg.createSVGPoint();
      pt.x = GAUGE_VIEWBOX_W / 2;
      pt.y = GAUGE_VIEWBOX_W / 2;
      const center = pt.matrixTransform(ctm);
      let angle = Math.atan2(e.clientY - center.y, e.clientX - center.x);
      if (angle < 0) angle += 2 * Math.PI;
      return angle;
    }
  } catch {
    // fall through to rect-based fallback
  }
  const rect = svg.getBoundingClientRect();
  const x = e.clientX - rect.left;
  const y = e.clientY - rect.top;
  const w = rect.width || 300;
  const h = rect.height || 300;
  return gaugeAngleFromOffset(x, y, w, h);
}

interface UseGaugeDragOptions {
  status:
    | "idle"
    | "charging"
    | "pending"
    | "conditioning"
    | "holding"
    | "error";
  onDragStart?: () => void;
  onDragEnd?: (current: number, target: number) => void;
  svgRef: React.RefObject<SVGSVGElement | null>;
}

interface PointerDragProps {
  onPointerDown: (e: React.PointerEvent<SVGSVGElement>) => void;
  onPointerMove: (e: React.PointerEvent<SVGSVGElement>) => void;
  onPointerUp: () => void;
  onPointerLeave: () => void;
}

interface UseGaugeDragResult {
  draggingGauge: "start" | "target" | null;
  hoveringMarker: boolean;
  pointerProps: PointerDragProps;
}

export function useGaugeDrag({
  status,
  onDragStart,
  onDragEnd,
  svgRef,
}: UseGaugeDragOptions): UseGaugeDragResult {
  const { isDragging, setDragging } = useGaugeStore();
  const draggingGauge = isDragging === "none" ? null : isDragging;
  const [hoveringMarker, setHoveringMarker] = useState(false);
  const draggingGaugeRef = useRef<"start" | "target" | null>(null);
  const dragInGapRef = useRef(false);
  const dragStartPercentRef = useRef(0);
  const dragMovedRef = useRef(false);
  const onDragEndRef = useRef(onDragEnd);
  const rafIdRef = useRef<number | null>(null);
  const scrollLockedRef = useRef(false);

  const lockScroll = useCallback(() => {
    if (scrollLockedRef.current) return;
    scrollLockedRef.current = true;
    document.documentElement.style.overflow = "hidden";
  }, []);

  const unlockScroll = useCallback(() => {
    if (!scrollLockedRef.current) return;
    scrollLockedRef.current = false;
    document.documentElement.style.overflow = "";
  }, []);

  useEffect(() => {
    return () => {
      if (rafIdRef.current) {
        cancelAnimationFrame(rafIdRef.current);
        rafIdRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    onDragEndRef.current = onDragEnd;
  }, [onDragEnd]);

  const HIT_RAD = HIT_TOLERANCE_DEG * RADIANS_PER_DEGREE;

  const getClosestMarker = useCallback(
    (mouseAngle: number) => {
      // Read percents from store directly to avoid stale values after drag updates.
      // Zustand subscription (currentPercent/targetPercent) isn't updated until next
      // React render, but getClosestMarker runs synchronously during pointerdown.
      const { currentPercent: cur, targetPercent: tgt } =
        useGaugeStore.getState();

      // When charging/conditioning/holding, only target is draggable
      if (
        status === "charging" ||
        status === "conditioning" ||
        status === "holding"
      ) {
        const dist = angularDistance(mouseAngle, percentageToAngle(tgt));
        return dist < HIT_RAD ? "target" : null;
      }

      const currentAngle = percentageToAngle(cur);
      const targetAngle = percentageToAngle(tgt);
      const currentDist = angularDistance(mouseAngle, currentAngle);
      const targetDist = angularDistance(mouseAngle, targetAngle);

      const markerSeparation = angularDistance(currentAngle, targetAngle);
      // When markers overlap, prefer target (more commonly dragged by users).
      if (markerSeparation === 0) {
        return targetDist < HIT_RAD || currentDist < HIT_RAD ? "target" : null;
      }

      // When markers are close (< 2×HIT_RAD), split the gap evenly so neither marker
      // "steals" clicks from the other. This prevents frustration when markers overlap.
      if (markerSeparation > 0 && markerSeparation < HIT_RAD * 2) {
        const halfSep = markerSeparation / 2;
        if (targetDist < halfSep) return "target";
        if (currentDist < halfSep) return "start";
        if (Math.min(currentDist, targetDist) < HIT_RAD) {
          return targetDist < currentDist ? "target" : "start";
        }
        return null;
      }

      // For well-separated markers, pick the closest one
      if (currentDist < HIT_RAD && targetDist < HIT_RAD) {
        return targetDist < currentDist ? "target" : "start";
      }
      if (currentDist < HIT_RAD) return "start";
      if (targetDist < HIT_RAD) return "target";
      return null;
    },
    [status, HIT_RAD],
  );

  const applyDragValue = useCallback(
    (dragging: "start" | "target", clamped: number) => {
      const { currentPercent: current, targetPercent: target } =
        useGaugeStore.getState();
      if (dragging === "start") {
        if (
          status === "charging" ||
          status === "conditioning" ||
          status === "holding"
        )
          return;
        // When current is dragged past target, auto-bump target to next 5% increment.
        // This prevents user from being stuck with target < current.
        const bumpCond =
          dragStartPercentRef.current <= target &&
          clamped > target &&
          target < 100;
        if (bumpCond) {
          const nextBucket = Math.ceil(clamped / 5) * 5;
          const bumpedTarget =
            nextBucket > clamped ? nextBucket : nextBucket + 5;
          const cappedBump = Math.min(bumpedTarget, 100);
          useGaugeStore.getState().setPercents(clamped, cappedBump);
        } else {
          const capped = Math.min(clamped, target);
          if (capped !== current) {
            useGaugeStore.getState().setPercents(capped, target);
          }
        }
      } else {
        if (
          status === "charging" ||
          status === "conditioning" ||
          status === "holding"
        ) {
          const eff = Math.max(clamped, current);
          if (eff !== target) {
            useGaugeStore.getState().setPercents(current, eff);
          }
        } else if (
          dragStartPercentRef.current >= current &&
          clamped <= current &&
          current > 0
        ) {
          useGaugeStore.getState().setPercents(clamped, clamped);
        } else if (clamped > current) {
          if (clamped !== target) {
            useGaugeStore.getState().setPercents(current, clamped);
          }
        }
      }
    },
    [status],
  );

  const applyDragFromAngle = useCallback(
    (mouseAngle: number) => {
      const dragging = draggingGaugeRef.current;
      if (!dragging) return;
      const { pct, inGap } = {
        pct: gaugeAngleToPercentage(mouseAngle, START_ANGLE_RAD, TOTAL_ARC_RAD),
        inGap: isAngleInGap(mouseAngle, START_ANGLE_RAD, TOTAL_ARC_RAD),
      };
      dragInGapRef.current = inGap;
      if (inGap) return;
      const snapped = snapToPercent(pct);
      const clamped = Math.max(0, Math.min(100, snapped));
      dragMovedRef.current = true;
      applyDragValue(dragging, clamped);
    },
    [applyDragValue],
  );

  const handlePointerDown = useCallback(
    (e: React.PointerEvent<SVGSVGElement>) => {
      const angle = getAngleFromPointer(e, e.currentTarget);
      const marker = getClosestMarker(angle);
      if (marker) {
        draggingGaugeRef.current = marker;
        setDragging(marker);
        dragInGapRef.current = false;
        dragMovedRef.current = false;
        if (e.pointerType === "touch") {
          lockScroll();
        }
        const store = useGaugeStore.getState();
        dragStartPercentRef.current =
          marker === "start" ? store.currentPercent : store.targetPercent;
        onDragStart?.();
        // Note: setPointerCapture removed - it interfered with global pointermove
        // listener after first drag, causing subsequent drags to fail (no pointermove
        // events fired on window). Global window listeners are sufficient since they're
        // gated by draggingGaugeRef.current.
      }
    },
    [onDragStart, getClosestMarker, setDragging, lockScroll],
  );

  const handlePointerMove = useCallback(
    (e: React.PointerEvent<SVGSVGElement>) => {
      const angle = getAngleFromPointer(e, e.currentTarget);
      const dragging = draggingGaugeRef.current;
      if (dragging) {
        if (rafIdRef.current) cancelAnimationFrame(rafIdRef.current);
        rafIdRef.current = requestAnimationFrame(() =>
          applyDragFromAngle(angle),
        );
        return;
      }
      const marker = getClosestMarker(angle);
      setHoveringMarker(!!marker);
    },
    [applyDragFromAngle, getClosestMarker],
  );

  const handlePointerUp = useCallback(() => {
    const dragging = draggingGaugeRef.current;
    const moved = dragMovedRef.current;
    if (dragging && dragInGapRef.current) {
      draggingGaugeRef.current = null;
      setDragging("none");
      dragStartPercentRef.current = 0;
      unlockScroll();
      if (moved) {
        const { currentPercent, targetPercent } = useGaugeStore.getState();
        onDragEndRef.current?.(currentPercent, targetPercent);
      }
      return;
    }
    if (dragging) {
      const store = useGaugeStore.getState();
      const pct =
        dragging === "target" ? store.targetPercent : store.currentPercent;
      applyDragValue(dragging, pct);
    }
    draggingGaugeRef.current = null;
    setDragging("none");
    dragStartPercentRef.current = 0;
    unlockScroll();
    if (moved) {
      const { currentPercent, targetPercent } = useGaugeStore.getState();
      onDragEndRef.current?.(currentPercent, targetPercent);
    }
  }, [applyDragValue, setDragging, unlockScroll]);

  const handlePointerLeave = useCallback(() => {
    if (draggingGaugeRef.current && dragMovedRef.current) {
      const { currentPercent, targetPercent } = useGaugeStore.getState();
      onDragEndRef.current?.(currentPercent, targetPercent);
    }
    draggingGaugeRef.current = null;
    setDragging("none");
    unlockScroll();
    setHoveringMarker(false);
  }, [setDragging, unlockScroll]);

  // Global drag handlers - added once, gated by ref
  useEffect(() => {
    const onMove = (e: PointerEvent) => {
      if (!draggingGaugeRef.current) return;
      const svg = svgRef.current;
      if (!svg) return;
      const angle = getAngleFromPointer(e, svg);
      if (rafIdRef.current) cancelAnimationFrame(rafIdRef.current);
      rafIdRef.current = requestAnimationFrame(() => applyDragFromAngle(angle));
    };
    const onUp = () => {
      if (rafIdRef.current) {
        cancelAnimationFrame(rafIdRef.current);
        rafIdRef.current = null;
      }
      const dragging = draggingGaugeRef.current;
      if (dragging && dragInGapRef.current) {
        draggingGaugeRef.current = null;
        setDragging("none");
        unlockScroll();
        setHoveringMarker(false);
        return;
      }
      if (dragging) {
        const store = useGaugeStore.getState();
        const pct =
          dragging === "target" ? store.targetPercent : store.currentPercent;
        applyDragValue(dragging, pct);
      }
      draggingGaugeRef.current = null;
      setDragging("none");
      unlockScroll();
      setHoveringMarker(false);
      if (dragMovedRef.current) {
        const { currentPercent, targetPercent } = useGaugeStore.getState();
        onDragEndRef.current?.(currentPercent, targetPercent);
      }
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    return () => {
      if (rafIdRef.current) {
        cancelAnimationFrame(rafIdRef.current);
        rafIdRef.current = null;
      }
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
    };
  }, [
    applyDragFromAngle,
    applyDragValue,
    setDragging,
    setHoveringMarker,
    svgRef,
    unlockScroll,
  ]);

  const pointerProps: PointerDragProps = {
    onPointerDown: handlePointerDown,
    onPointerMove: handlePointerMove,
    onPointerUp: handlePointerUp,
    onPointerLeave: handlePointerLeave,
  };

  return {
    draggingGauge,
    hoveringMarker,
    pointerProps,
  };
}
