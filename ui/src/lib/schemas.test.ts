import {
  VehicleSchema,
  HistoryVehicleSchema,
  HistoryChargeSessionSchema,
  ChargeSessionResponseSchema,
  ScheduleSchema,
} from "@/lib/schemas";
import { describe, it, expect } from "vitest";

describe("VehicleSchema", () => {
  it("parses valid vehicle", () => {
    const vehicle = VehicleSchema.parse({
      id: "rm1",
      name: "Maeving RM1S",
      capacityKwh: 3.8,
      chargerOutputW: 1200,
      chargingEfficiency: 0.8,
      currentPercent: 30,
      targetPercent: 80,
      rangeMinMi: 0,
      rangeMaxMi: 30,
    });
    expect(vehicle.id).toBe("rm1");
    expect(vehicle.capacityKwh).toBe(3.8);
  });

  it("parses vehicle without optional fields", () => {
    const vehicle = VehicleSchema.parse({
      id: "rm1",
      name: "Maeving RM1S",
      capacityKwh: 3.8,
      chargerOutputW: 1200,
      chargingEfficiency: 0.8,
      rangeMinMi: 0,
      rangeMaxMi: 30,
    });
    expect(vehicle.currentPercent).toBeUndefined();
    expect(vehicle.targetPercent).toBeUndefined();
  });

  it("rejects vehicle missing required field", () => {
    expect(() =>
      VehicleSchema.parse({
        id: "rm1",
        name: "Maeving RM1S",
        capacityKwh: 3.8,
        chargerOutputW: 1200,
        chargingEfficiency: 0.8,
      }),
    ).toThrow();
  });

  it("rejects vehicle with wrong types", () => {
    expect(() =>
      VehicleSchema.parse({
        id: "rm1",
        name: "Maeving RM1S",
        capacityKwh: "3.8",
        chargerOutputW: 1200,
        chargingEfficiency: 0.8,
        rangeMinMi: 0,
        rangeMaxMi: 30,
      }),
    ).toThrow();
  });
});

describe("HistoryVehicleSchema", () => {
  it("parses valid history vehicle", () => {
    const vehicle = HistoryVehicleSchema.parse({
      id: "rm1",
      name: "Maeving RM1S",
      capacityKwh: 3.8,
      chargerOutputW: 1200,
      chargingEfficiency: 0.8,
      rangeMinMi: 100,
      rangeMaxMi: 150,
    });
    expect(vehicle.id).toBe("rm1");
  });
});

describe("HistoryChargeSessionSchema", () => {
  it("parses valid session", () => {
    const session = HistoryChargeSessionSchema.parse({
      id: "session-1",
      vehicleId: "rm1",
      createdAt: "2025-01-01T03:00:00Z",
      endedAt: "2025-01-01T04:00:00Z",
      startKwh: 10,
      endKwh: 20,
      startPercent: 30,
      endPercent: 80,
      targetKwh: 20,
      targetPercent: 80,
      status: "completed",
      totalBatteryKwh: 12.5,
    });
    expect(session.id).toBe("session-1");
    expect(session.status).toBe("completed");
  });

  it("parses session without optional fields", () => {
    const session = HistoryChargeSessionSchema.parse({
      id: "session-1",
      vehicleId: "rm1",
      createdAt: "2025-01-01T03:00:00Z",
      startKwh: 10,
      startPercent: 30,
      targetKwh: 20,
      targetPercent: 80,
      status: "active",
    });
    expect(session.endedAt).toBeUndefined();
    expect(session.endKwh).toBeUndefined();
  });

  it("rejects session missing required field", () => {
    expect(() =>
      HistoryChargeSessionSchema.parse({
        id: "session-1",
        vehicleId: "rm1",
        createdAt: "2025-01-01T03:00:00Z",
        startKwh: 10,
        startPercent: 30,
        targetKwh: 20,
        targetPercent: 80,
      }),
    ).toThrow();
  });
});

describe("ChargeSessionResponseSchema", () => {
  it("parses valid response", () => {
    const data = ChargeSessionResponseSchema.parse({
      id: "session-1",
      vehicleId: "vehicle-1",
      createdAt: "2024-01-01T10:00:00Z",
      status: "active",
      powerDraw: 600,
      currentPercent: 50,
      startPercent: 30,
      targetPercent: 80,
      startKwh: 10,
      targetKwh: 15,
    });
    expect(data.status).toBe("active");
    expect(data.powerDraw).toBe(600);
  });

  it("parses response with minimal fields", () => {
    const data = ChargeSessionResponseSchema.parse({
      id: "session-1",
      vehicleId: "vehicle-1",
      createdAt: "2024-01-01T10:00:00Z",
      status: "idle",
      startKwh: 10,
      targetKwh: 15,
      startPercent: 30,
      targetPercent: 80,
    });
    expect(data.status).toBe("idle");
    expect(data.powerDraw).toBeUndefined();
  });

  it("parses response with null optional fields", () => {
    const data = ChargeSessionResponseSchema.parse({
      id: "session-1",
      vehicleId: "vehicle-1",
      createdAt: "2024-01-01T10:00:00Z",
      status: "idle",
      powerDraw: null,
      startKwh: 10,
      targetKwh: 15,
      startPercent: 30,
      targetPercent: 80,
    });
    expect(data.powerDraw).toBeNull();
  });

  it("rejects response missing status", () => {
    expect(() =>
      ChargeSessionResponseSchema.parse({
        id: "session-1",
        vehicleId: "vehicle-1",
        createdAt: "2024-01-01T10:00:00Z",
        powerDraw: 600,
        startKwh: 10,
        targetKwh: 15,
        startPercent: 30,
        targetPercent: 80,
      }),
    ).toThrow();
  });

  it("parses response with estimatedResumeTime", () => {
    const data = ChargeSessionResponseSchema.parse({
      id: "session-1",
      vehicleId: "vehicle-1",
      createdAt: "2024-01-01T10:00:00Z",
      status: "holding",
      startKwh: 10,
      targetKwh: 15,
      startPercent: 30,
      targetPercent: 80,
      estimatedResumeTime: "23:30",
    });
    expect(data.estimatedResumeTime).toBe("23:30");
  });

  it("rejects malformed estimatedResumeTime", () => {
    expect(() =>
      ChargeSessionResponseSchema.parse({
        id: "session-1",
        vehicleId: "vehicle-1",
        createdAt: "2024-01-01T10:00:00Z",
        status: "holding",
        startKwh: 10,
        targetKwh: 15,
        startPercent: 30,
        targetPercent: 80,
        estimatedResumeTime: "not-a-time",
      }),
    ).toThrow();
  });
});

describe("ScheduleSchema", () => {
  it("parses valid daily schedule", () => {
    const schedule = ScheduleSchema.parse({
      id: "plug",
      time: "03:00",
      enabled: true,
    });
    expect(schedule.id).toBe("plug");
    expect(schedule.time).toBe("03:00");
    expect(schedule.enabled).toBe(true);
  });

  it("defaults type to 'daily' when not provided", () => {
    const schedule = ScheduleSchema.parse({
      id: "plug",
      time: "03:00",
      enabled: true,
    });
    expect(schedule.type).toBe("daily");
  });

  it("parses carbon_aware schedule with window fields", () => {
    const schedule = ScheduleSchema.parse({
      id: "plug",
      type: "carbon_aware",
      time: "22:00",
      windowStart: "22:00",
      windowEnd: "06:00",
      enabled: true,
    });
    expect(schedule.type).toBe("carbon_aware");
    expect(schedule.windowStart).toBe("22:00");
    expect(schedule.windowEnd).toBe("06:00");
    expect(schedule.enabled).toBe(true);
  });

  it("parses carbon_aware without optional window fields", () => {
    const schedule = ScheduleSchema.parse({
      id: "plug",
      type: "carbon_aware",
      time: "22:00",
      enabled: false,
    });
    expect(schedule.type).toBe("carbon_aware");
    expect(schedule.windowStart).toBeUndefined();
    expect(schedule.windowEnd).toBeUndefined();
  });

  it("parses daily schedule with readyBy", () => {
    const schedule = ScheduleSchema.parse({
      id: "plug",
      time: "01:00",
      readyBy: "07:00",
      enabled: true,
    });
    expect(schedule.readyBy).toBe("07:00");
  });

  it("parses daily schedule without readyBy", () => {
    const schedule = ScheduleSchema.parse({
      id: "plug",
      time: "01:00",
      enabled: true,
    });
    expect(schedule.readyBy).toBeUndefined();
  });

  it("rejects malformed readyBy", () => {
    expect(() =>
      ScheduleSchema.parse({
        id: "plug",
        time: "01:00",
        readyBy: "not-a-time",
        enabled: true,
      }),
    ).toThrow();
  });

  it("parses carbon_aware schedule with twoStage", () => {
    const schedule = ScheduleSchema.parse({
      id: "plug",
      type: "carbon_aware",
      time: "22:00",
      windowStart: "22:00",
      windowEnd: "06:00",
      twoStage: true,
      enabled: true,
    });
    expect(schedule.twoStage).toBe(true);
  });

  it("parses carbon_aware schedule with estimatedPlan", () => {
    const schedule = ScheduleSchema.parse({
      id: "plug",
      type: "carbon_aware",
      time: "22:00",
      windowStart: "22:00",
      windowEnd: "06:00",
      twoStage: true,
      estimatedPlan: {
        stage1Start: "22:30",
        stage1End: "23:15",
        stage2Start: "04:50",
        stage2End: "05:35",
      },
      enabled: true,
    });
    expect(schedule.estimatedPlan).toEqual({
      stage1Start: "22:30",
      stage1End: "23:15",
      stage2Start: "04:50",
      stage2End: "05:35",
    });
  });

  it("rejects malformed estimatedPlan", () => {
    expect(() =>
      ScheduleSchema.parse({
        id: "plug",
        type: "carbon_aware",
        time: "22:00",
        windowStart: "22:00",
        windowEnd: "06:00",
        twoStage: true,
        estimatedPlan: {
          stage1Start: "not-a-time",
          stage1End: "23:15",
          stage2Start: "04:50",
          stage2End: "05:35",
        },
        enabled: true,
      }),
    ).toThrow();
  });

  it("rejects invalid schedule type", () => {
    expect(() =>
      ScheduleSchema.parse({
        id: "plug",
        type: "weekly",
        time: "03:00",
        enabled: true,
      }),
    ).toThrow();
  });

  it("rejects schedule missing required field", () => {
    expect(() =>
      ScheduleSchema.parse({
        id: "plug",
        time: "03:00",
      }),
    ).toThrow();
  });
});
