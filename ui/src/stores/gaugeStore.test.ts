import { describe, it, expect, beforeEach } from "vitest";

import { useGaugeStore } from "./gaugeStore";

describe("gaugeStore", () => {
  beforeEach(() => {
    useGaugeStore.setState({
      currentPercent: 20,
      targetPercent: 80,
      isDragging: "none",
      initialized: false,
    });
  });

  describe("initial state", () => {
    it("defaults currentPercent to 20", () => {
      expect(useGaugeStore.getState().currentPercent).toBe(20);
    });

    it("defaults targetPercent to 80", () => {
      expect(useGaugeStore.getState().targetPercent).toBe(80);
    });

    it("defaults isDragging to 'none'", () => {
      expect(useGaugeStore.getState().isDragging).toBe("none");
    });
  });

  describe("setCurrentPercent", () => {
    it("updates currentPercent", () => {
      useGaugeStore.getState().setCurrentPercent(45);
      expect(useGaugeStore.getState().currentPercent).toBe(45);
    });

    it("does not affect targetPercent", () => {
      useGaugeStore.getState().setCurrentPercent(45);
      expect(useGaugeStore.getState().targetPercent).toBe(80);
    });
  });

  describe("setTargetPercent", () => {
    it("updates targetPercent", () => {
      useGaugeStore.getState().setTargetPercent(90);
      expect(useGaugeStore.getState().targetPercent).toBe(90);
    });

    it("does not affect currentPercent", () => {
      useGaugeStore.getState().setTargetPercent(90);
      expect(useGaugeStore.getState().currentPercent).toBe(20);
    });
  });

  describe("setDragging", () => {
    it("sets isDragging to 'start'", () => {
      useGaugeStore.getState().setDragging("start");
      expect(useGaugeStore.getState().isDragging).toBe("start");
    });

    it("sets isDragging to 'target'", () => {
      useGaugeStore.getState().setDragging("target");
      expect(useGaugeStore.getState().isDragging).toBe("target");
    });

    it("sets isDragging to 'none'", () => {
      useGaugeStore.getState().setDragging("start");
      useGaugeStore.getState().setDragging("none");
      expect(useGaugeStore.getState().isDragging).toBe("none");
    });
  });

  describe("setPercents", () => {
    it("updates both currentPercent and targetPercent", () => {
      useGaugeStore.getState().setPercents(30, 70);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(30);
      expect(state.targetPercent).toBe(70);
    });

    it("does not affect isDragging", () => {
      useGaugeStore.getState().setDragging("start");
      useGaugeStore.getState().setPercents(30, 70);
      expect(useGaugeStore.getState().isDragging).toBe("start");
    });
  });

  describe("invariant enforcement", () => {
    it("setCurrentPercent clamps to targetPercent", () => {
      useGaugeStore.getState().setCurrentPercent(90);
      expect(useGaugeStore.getState().currentPercent).toBe(80);
    });

    it("setCurrentPercent clamps to 0 floor", () => {
      useGaugeStore.getState().setCurrentPercent(-10);
      expect(useGaugeStore.getState().currentPercent).toBe(0);
    });

    it("setTargetPercent clamps to currentPercent floor", () => {
      useGaugeStore.getState().setTargetPercent(10);
      expect(useGaugeStore.getState().targetPercent).toBe(20);
    });

    it("setTargetPercent clamps to 100 ceiling", () => {
      useGaugeStore.getState().setTargetPercent(110);
      expect(useGaugeStore.getState().targetPercent).toBe(100);
    });

    it("setPercents enforces current <= target", () => {
      useGaugeStore.getState().setPercents(90, 50);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(50);
      expect(state.targetPercent).toBe(50);
    });

    it("setPercents clamps current to 0 floor", () => {
      useGaugeStore.getState().setPercents(-10, 50);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(0);
      expect(state.targetPercent).toBe(50);
    });

    it("setPercents clamps target to 100 ceiling", () => {
      useGaugeStore.getState().setPercents(30, 110);
      const state = useGaugeStore.getState();
      expect(state.currentPercent).toBe(30);
      expect(state.targetPercent).toBe(100);
    });
  });

  describe("initialized", () => {
    it("defaults to false", () => {
      expect(useGaugeStore.getState().initialized).toBe(false);
    });

    it("markInitialized sets initialized to true", () => {
      useGaugeStore.getState().markInitialized();
      expect(useGaugeStore.getState().initialized).toBe(true);
    });
  });

  describe("getState", () => {
    it("returns the full state object", () => {
      const state = useGaugeStore.getState();
      expect(state).toHaveProperty("currentPercent");
      expect(state).toHaveProperty("targetPercent");
      expect(state).toHaveProperty("isDragging");
      expect(state).toHaveProperty("setCurrentPercent");
      expect(state).toHaveProperty("setTargetPercent");
      expect(state).toHaveProperty("setDragging");
      expect(state).toHaveProperty("setPercents");
      expect(state).toHaveProperty("initialized");
      expect(state).toHaveProperty("markInitialized");
    });
  });
});
