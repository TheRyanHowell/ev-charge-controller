import { z } from "zod";

export const VehicleModelSchema = z.object({
  id: z.string(),
  name: z.string(),
  capacityKwh: z.number(),
  chargerOutputW: z.number(),
  chargingEfficiency: z.number(),
  time0to100Min: z.number().optional(),
  time0to80Min: z.number().optional(),
  time20to80Min: z.number().optional(),
  time20to100Min: z.number().optional(),
  packVoltageMaxV: z.number().optional(),
  packCutoffCurrentMa: z.number().optional(),
  rangeMinMi: z.number(),
  rangeMaxMi: z.number(),
});

export const VehicleSchema = z.object({
  id: z.string(),
  modelId: z.string().optional(),
  name: z.string(),
  modelName: z.string().optional(),
  capacityKwh: z.number(),
  chargerOutputW: z.number(),
  chargingEfficiency: z.number(),
  time0to100Min: z.number().optional(),
  time0to80Min: z.number().optional(),
  time20to80Min: z.number().optional(),
  time20to100Min: z.number().optional(),
  // Optional pack electrical specs for physics-based P100 fallback in the
  // CC/CV curve algorithm. Used when no CV charge-time data is available.
  // P100 = packVoltageMaxV (V) × packCutoffCurrentMa (mA) / 1_000_000 (kW)
  packVoltageMaxV: z.number().optional(),
  packCutoffCurrentMa: z.number().optional(),
  currentPercent: z.number().optional(),
  targetPercent: z.number().optional(),
  rangeMinMi: z.number(),
  rangeMaxMi: z.number(),
  // Pre-computed lifetime stats
  totalSessions: z.number().optional(),
  totalBatteryKwh: z.number().optional(),
  totalWallKwh: z.number().optional(),
  totalCo2Grams: z.number().optional(),
  totalCostPence: z.number().optional(),
  minSessionBatteryKwh: z.number().optional().default(0),
  maxSessionBatteryKwh: z.number().optional().default(0),
  lastSessionAt: z.string().optional().nullable(),
  // Per-vehicle notification preferences (default true = opted in)
  notifyChargeStarted: z.boolean().default(true),
  notifyChargeComplete: z.boolean().default(true),
  notifyChargerOffline: z.boolean().default(true),
  notifyMaintenanceOffline: z.boolean().default(true),
});

export const HistoryVehicleSchema = VehicleSchema.pick({
  id: true,
  name: true,
  capacityKwh: true,
  chargingEfficiency: true,
  chargerOutputW: true,
  time0to100Min: true,
  time0to80Min: true,
  time20to80Min: true,
  time20to100Min: true,
  packVoltageMaxV: true,
  packCutoffCurrentMa: true,
  rangeMinMi: true,
  rangeMaxMi: true,
});

export const HistoryChargeSessionSchema = z.object({
  id: z.string(),
  vehicleId: z.string(),
  createdAt: z.string(),
  endedAt: z.string().optional(),
  startKwh: z.number(),
  endKwh: z.number().optional(),
  startPercent: z.number(),
  endPercent: z.number().optional(),
  targetKwh: z.number(),
  targetPercent: z.number(),
  status: z.string(),
  totalBatteryKwh: z.number().optional(),
  avgCarbonIntensityGCo2PerKwh: z.number().optional(),
  // Pre-computed per-session stats (NULL for old sessions)
  batteryKwh: z.number().optional().nullable(),
  wallKwh: z.number().optional().nullable(),
  avgCarbonIntensity: z.number().optional().nullable(),
  co2Grams: z.number().optional().nullable(),
  // Frozen electricity cost (pence) and off-peak wall energy share.
  costPence: z.number().optional().nullable(),
  offPeakKwh: z.number().optional().nullable(),
});

export const ChargeSessionResponseSchema = z.object({
  id: z.string(),
  vehicleId: z.string(),
  createdAt: z.string(),
  startedAt: z.string().optional().nullable(),
  endedAt: z.string().optional().nullable(),
  startKwh: z.number(),
  endKwh: z.number().optional().nullable(),
  targetKwh: z.number(),
  startPercent: z.number(),
  endPercent: z.number().optional().nullable(),
  targetPercent: z.number(),
  status: z.string(),
  startTotalKwh: z.number().optional().nullable(),
  lastBlendedKwh: z.number().optional().nullable(),
  powerDraw: z.number().optional().nullable(),
  currentPercent: z.number().optional().nullable(),
  energyAddedKwh: z.number().optional().nullable(),
  voltage: z.number().optional().nullable(),
  current: z.number().optional().nullable(),
  // Pre-computed per-session stats (NULL for old sessions)
  batteryKwh: z.number().optional().nullable(),
  wallKwh: z.number().optional().nullable(),
  avgCarbonIntensity: z.number().optional().nullable(),
  co2Grams: z.number().optional().nullable(),
  // Frozen electricity cost (pence) and off-peak wall energy share.
  costPence: z.number().optional().nullable(),
  offPeakKwh: z.number().optional().nullable(),
});

const timePattern = /^([01]\d|2[0-3]):[0-5]\d$/;

export const ScheduleSchema = z.object({
  id: z.string(),
  type: z.enum(["daily", "carbon_aware"]).default("daily"),
  time: z.string().regex(timePattern),
  windowStart: z.string().regex(timePattern).optional().nullable(),
  windowEnd: z.string().regex(timePattern).optional().nullable(),
  enabled: z.boolean(),
});

const OffPeakWindowSchema = z.object({
  start: z.string().regex(timePattern),
  end: z.string().regex(timePattern),
  ratePence: z.number().nonnegative(),
});

export const TariffSettingsSchema = z.object({
  baseRatePence: z.number().nonnegative(),
  offPeakWindows: OffPeakWindowSchema.array().default([]),
  updatedAt: z.string().optional().nullable(),
});

export const PlugSchema = z.object({
  id: z.string(),
  userId: z.string(),
  name: z.string(),
  namespace: z.string(),
  mqttTopic: z.string(),
  tls: z.boolean(),
  online: z.boolean(),
  initialized: z.boolean().optional().default(false),
  type: z.enum(["charging", "maintenance"]).default("charging"),
  powerOn: z.boolean().default(false),
  lastSeen: z.string().optional().nullable(),
  vehicleId: z.string().optional().nullable(),
  createdAt: z.string(),
});

export const ProvisioningResultSchema = z.object({
  plug: PlugSchema,
});

export const ConsoleCommandsResultSchema = z.object({
  consoleCommands: z.string(),
});

export const CarbonIntensitySchema = z.object({
  forecast: z.number(),
  actual: z.number(),
  index: z.string(),
});

export const PowerReadingSchema = z.object({
  id: z.string(),
  sessionId: z.string(),
  timestamp: z.string(),
  voltage: z.number(),
  current: z.number(),
  power: z.number(),
  energyKwh: z.number(),
  carbonIntensityGCo2PerKwh: z.number().optional(),
});

export const SocSnapshotSchema = z.object({
  id: z.string(),
  sessionId: z.string(),
  timestamp: z.string(),
  socPercent: z.number(),
});

// CCVProfile is derived client-side (not fetched), but defining a schema lets the
// CC/CV chart satisfy LineChart's required `schema` prop without weakening it.
export const CCVProfileSchema = z.object({
  soc: z.number(),
  power: z.number(),
});

export const InitialChargeSessionSchema = ChargeSessionResponseSchema.pick({
  status: true,
  powerDraw: true,
  startPercent: true,
  currentPercent: true,
  targetPercent: true,
  startedAt: true,
  voltage: true,
  current: true,
  energyAddedKwh: true,
}).extend({
  renderTimeMs: z.number(),
});

const DailyEnergySchema = z.object({
  date: z.string(),
  batteryKwh: z.number(),
  sessionCount: z.number(),
  co2Grams: z.number(),
  avgCarbonIntensityGCo2PerKwh: z.number().optional(),
});

export const VehicleStatsSchema = z.object({
  totalSessions: z.number(),
  totalBatteryKwh: z.number(),
  avgSessionKwh: z.number(),
  avgCarbonGCo2PerKwh: z.number().optional(),
  totalCo2Grams: z.number(),
  totalCostPence: z.number().optional(),
  avgCostPence: z.number().optional(),
  minSessionBatteryKwh: z.number().optional().default(0),
  maxSessionBatteryKwh: z.number().optional().default(0),
  dailyEnergy: DailyEnergySchema.array(),
});

export type VehicleModel = z.infer<typeof VehicleModelSchema>;
export type Vehicle = z.infer<typeof VehicleSchema>;
export type HistoryVehicle = z.infer<typeof HistoryVehicleSchema>;
export type HistoryChargeSession = z.infer<typeof HistoryChargeSessionSchema>;
export type ChargeSessionResponse = z.infer<typeof ChargeSessionResponseSchema>;
export type InitialChargeSession = z.infer<typeof InitialChargeSessionSchema>;
export type Schedule = z.infer<typeof ScheduleSchema>;
export type CarbonIntensity = z.infer<typeof CarbonIntensitySchema>;
export type PowerReading = z.infer<typeof PowerReadingSchema>;
export type SOCSnapshot = z.infer<typeof SocSnapshotSchema>;
export type CCVProfile = z.infer<typeof CCVProfileSchema>;
export type Plug = z.infer<typeof PlugSchema>;
export type ProvisioningResult = z.infer<typeof ProvisioningResultSchema>;
export type VehicleStats = z.infer<typeof VehicleStatsSchema>;

export type TariffSettings = z.infer<typeof TariffSettingsSchema>;
