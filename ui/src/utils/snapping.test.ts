import { describe, it, expect } from "vitest";

import { snapPercentage, snapToPercent } from "./snapping";

describe("snapPercentage", () => {
  it("snaps 4% to 5% (2% from 5%)", () => {
    expect(snapPercentage(4)).toBe(5);
  });

  it("snaps 7% to 5% (2% from 5%)", () => {
    expect(snapPercentage(7)).toBe(5);
  });

  it("snaps 12% to 10% (2% from 10%)", () => {
    expect(snapPercentage(12)).toBe(10);
  });

  it("snaps 13% to 15% (2% from 15%)", () => {
    expect(snapPercentage(13)).toBe(15);
  });

  it("snaps 18% to 20% (2% from 20%)", () => {
    expect(snapPercentage(18)).toBe(20);
  });

  it("snaps 23% to 25% (2% from 25%)", () => {
    expect(snapPercentage(23)).toBe(25);
  });

  it("snaps 0% to 0%", () => {
    expect(snapPercentage(0)).toBe(0);
  });

  it("snaps 5% to 5%", () => {
    expect(snapPercentage(5)).toBe(5);
  });

  it("snaps 10% to 10%", () => {
    expect(snapPercentage(10)).toBe(10);
  });

  it("snaps 100% to 100%", () => {
    expect(snapPercentage(100)).toBe(100);
  });

  it("snaps 2.5% to 0% (boundary: exactly 2.5% from 0%)", () => {
    expect(snapPercentage(2.5)).toBe(0);
  });

  it("snaps 12.5% to 10% (boundary: 2.5% from 10%)", () => {
    expect(snapPercentage(12.5)).toBe(10);
  });

  it("snaps 17.5% to 20% (boundary: 2.5% from 20%)", () => {
    expect(snapPercentage(17.5)).toBe(20);
  });

  it("snaps 8% to 10% (within 2.5% of 10%)", () => {
    expect(snapPercentage(8)).toBe(10);
  });

  it("snaps 22% to 20% (within 2.5% of 20%)", () => {
    expect(snapPercentage(22)).toBe(20);
  });
});

describe("snapToPercent", () => {
  it("snaps 4.3% to 4%", () => {
    expect(snapToPercent(4.3)).toBe(4);
  });

  it("snaps 4.6% to 5%", () => {
    expect(snapToPercent(4.6)).toBe(5);
  });

  it("snaps 7.5% to 8%", () => {
    expect(snapToPercent(7.5)).toBe(8);
  });

  it("snaps 12.1% to 12%", () => {
    expect(snapToPercent(12.1)).toBe(12);
  });

  it("snaps 13.9% to 14%", () => {
    expect(snapToPercent(13.9)).toBe(14);
  });

  it("snaps 0.4% to 0%", () => {
    expect(snapToPercent(0.4)).toBe(0);
  });

  it("snaps 99.6% to 100%", () => {
    expect(snapToPercent(99.6)).toBe(100);
  });

  it("keeps exact percentages", () => {
    expect(snapToPercent(5)).toBe(5);
    expect(snapToPercent(10)).toBe(10);
    expect(snapToPercent(25)).toBe(25);
    expect(snapToPercent(50)).toBe(50);
    expect(snapToPercent(75)).toBe(75);
    expect(snapToPercent(100)).toBe(100);
  });

  it("snaps negative values", () => {
    expect(snapToPercent(-0.6)).toBe(-1);
  });

  it("snaps values above 100", () => {
    expect(snapToPercent(100.4)).toBe(100);
    expect(snapToPercent(100.6)).toBe(101);
  });

  it("snaps very small values", () => {
    expect(snapToPercent(0.1)).toBe(0);
    expect(snapToPercent(0.01)).toBe(0);
    expect(snapToPercent(0.5)).toBe(1);
  });
});

describe("snapPercentage edge cases", () => {
  it("clamps negative values to 0", () => {
    expect(snapPercentage(-3)).toBe(0);
  });

  it("clamps values above 100 to 100", () => {
    expect(snapPercentage(101)).toBe(100);
    expect(snapPercentage(107)).toBe(100);
  });

  it("snaps exact 5% boundaries", () => {
    expect(snapPercentage(15)).toBe(15);
    expect(snapPercentage(25)).toBe(25);
    expect(snapPercentage(35)).toBe(35);
  });
});
