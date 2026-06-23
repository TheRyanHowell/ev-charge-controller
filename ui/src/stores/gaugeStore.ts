import { create } from "zustand";

type DraggingState = "none" | "start" | "target";

interface GaugeState {
  currentPercent: number;
  targetPercent: number;
  isDragging: DraggingState;
  initialized: boolean;

  setCurrentPercent: (value: number) => void;
  setTargetPercent: (value: number) => void;
  setDragging: (state: DraggingState) => void;
  setPercents: (current: number, target: number) => void;
  markInitialized: () => void;
  reset: () => void;
}

const clamp = (v: number, min: number, max: number) =>
  Math.max(min, Math.min(max, v));

export const useGaugeStore = create<GaugeState>()((set, get) => ({
  currentPercent: 20,
  targetPercent: 80,
  isDragging: "none",
  initialized: false,

  setCurrentPercent: (value) => {
    const { targetPercent } = get();
    const clamped = clamp(value, 0, targetPercent);
    set({ currentPercent: clamped });
  },

  setTargetPercent: (value) => {
    const { currentPercent } = get();
    const clamped = clamp(value, currentPercent, 100);
    set({ targetPercent: clamped });
  },

  setDragging: (state) => set({ isDragging: state }),

  setPercents: (current, target) => {
    const t = clamp(target, 0, 100);
    const c = clamp(current, 0, t);
    set({ currentPercent: c, targetPercent: t });
  },

  markInitialized: () => set({ initialized: true }),

  reset: () =>
    set({
      currentPercent: 20,
      targetPercent: 80,
      isDragging: "none",
      initialized: false,
    }),
}));
