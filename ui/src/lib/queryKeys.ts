const ALL = ["chargeController"] as const;

export const queryKeys = {
  all: ALL,

  chargeSession: {
    all: [...ALL, "chargeSession"] as const,
    byVehicle: (vehicleId: string) =>
      [...ALL, "chargeSession", vehicleId] as const,
    none: [...ALL, "chargeSession", "none"] as const,
  },

  vehicleModels: {
    all: [...ALL, "vehicleModels"] as const,
  },

  vehicles: {
    all: [...ALL, "vehicles"] as const,
    byId: (vehicleId: string) => [...ALL, "vehicles", vehicleId] as const,
    stats: (vehicleId: string) =>
      [...ALL, "vehicles", vehicleId, "stats"] as const,
  },

  history: {
    vehicles: [...ALL, "historyVehicles"] as const,
    sessions: (vehicleId: string | null, date: string | null) =>
      [...ALL, "historySessions", vehicleId, date] as const,
    allSessions: [...ALL, "historySessions"] as const,
  },

  schedule: {
    all: [...ALL, "schedule"] as const,
  },

  tariff: {
    settings: [...ALL, "tariff", "settings"] as const,
  },

  carbonIntensity: {
    current: [...ALL, "carbonIntensity", "current"] as const,
  },

  lineChart: {
    byEndpoint: (endpoint: string, vehicleId: string | null) =>
      [...ALL, "lineChart", endpoint, vehicleId] as const,
  },

  plugs: {
    all: [...ALL, "plugs"] as const,
    byId: (plugId: string) => [...ALL, "plugs", plugId] as const,
    schedule: (plugId: string) =>
      [...ALL, "plugs", plugId, "schedule"] as const,
    provisioning: (plugId: string) =>
      [...ALL, "plugs", plugId, "provisioning"] as const,
  },
};
