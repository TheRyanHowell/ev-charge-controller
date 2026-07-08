import type {
  CarbonIntensity,
  ChargeSessionResponse,
  HistoryChargeSession,
  HistoryVehicle,
  Plug,
  SOCSnapshot,
  Vehicle,
  VehicleModel,
} from "@/lib/schemas";

/* ------------------------------------------------------------------ */
/*  Base factory defaults - match production seed where applicable     */
/* ------------------------------------------------------------------ */

const defaultVehicleModel: VehicleModel = {
  id: "rm1",
  name: "Maeving RM1",
  capacityKwh: 2.026,
  chargerOutputW: 600,
  chargingEfficiency: 0.8,
  rangeMinMi: 0,
  rangeMaxMi: 0,
};

const defaultVehicle: Vehicle = {
  ...defaultVehicleModel,
  id: "rm1",
  name: "Maeving RM1",
  modelName: "Maeving RM1",
  notifyChargeStarted: true,
  notifyChargeComplete: true,
  notifyChargerOffline: true,
  notifyMaintenanceOffline: true,
  minSessionBatteryKwh: 0,
  maxSessionBatteryKwh: 0,
};

const defaultHistoryVehicle: HistoryVehicle = {
  id: "rm1",
  name: "Maeving RM1",
  capacityKwh: 2.026,
  chargerOutputW: 600,
  chargingEfficiency: 0.8,
  rangeMinMi: 0,
  rangeMaxMi: 0,
};

const defaultChargeSession: ChargeSessionResponse = {
  id: "session-1",
  vehicleId: "rm1",
  createdAt: "2024-01-01T10:00:00Z",
  status: "pending",
  startKwh: 0.4,
  targetKwh: 1.621,
  startPercent: 20,
  targetPercent: 80,
};

const defaultHistorySession: HistoryChargeSession = {
  id: "session-1",
  vehicleId: "rm1",
  createdAt: "2024-01-15T10:00:00Z",
  endedAt: "2024-01-15T12:30:00Z",
  startKwh: 0.4,
  endKwh: 1.621,
  targetKwh: 2.026,
  startPercent: 20,
  endPercent: 80,
  targetPercent: 100,
  status: "completed",
  totalBatteryKwh: 1.221,
};

const defaultPlug: Plug = {
  id: "plug-1",
  userId: "user-1",
  name: "Test Plug",
  namespace: "default-ns",
  mqttTopic: "test/topic",
  tls: false,
  online: true,
  initialized: true,
  type: "charging",
  powerOn: false,
  createdAt: "2024-01-01T00:00:00Z",
};

const defaultSOCSnapshot: SOCSnapshot = {
  id: "soc-1",
  sessionId: "session-1",
  timestamp: "2024-01-15T10:05:00Z",
  socPercent: 25,
};

const defaultCarbonIntensity: CarbonIntensity = {
  forecast: 200,
  actual: 180,
  index: "2024-01-15T10:00:00Z",
};

/* ------------------------------------------------------------------ */
/*  Factory functions                                                  */
/* ------------------------------------------------------------------ */

export function createVehicleModel(
  overrides: Partial<VehicleModel> = {},
): VehicleModel {
  return { ...defaultVehicleModel, ...overrides };
}

export function createVehicle(overrides: Partial<Vehicle> = {}): Vehicle {
  return { ...defaultVehicle, ...overrides };
}

export function createHistoryVehicle(
  overrides: Partial<HistoryVehicle> = {},
): HistoryVehicle {
  return { ...defaultHistoryVehicle, ...overrides };
}

export function createChargeSession(
  overrides: Partial<ChargeSessionResponse> = {},
): ChargeSessionResponse {
  return { ...defaultChargeSession, ...overrides };
}

export function createHistorySession(
  overrides: Partial<HistoryChargeSession> = {},
): HistoryChargeSession {
  return { ...defaultHistorySession, ...overrides } as HistoryChargeSession;
}

export function createPlug(overrides: Partial<Plug> = {}): Plug {
  return { ...defaultPlug, ...overrides };
}

export function createSOCSnapshot(
  overrides: Partial<SOCSnapshot> = {},
): SOCSnapshot {
  return { ...defaultSOCSnapshot, ...overrides };
}

export function createCarbonIntensity(
  overrides: Partial<CarbonIntensity> = {},
): CarbonIntensity {
  return { ...defaultCarbonIntensity, ...overrides };
}
