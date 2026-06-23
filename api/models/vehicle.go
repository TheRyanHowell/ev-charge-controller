package models

import "time"

// Vehicle is a per-user instance of a VehicleModel. It holds state
// (current/target SoC) and references the catalog model for config.
// The API response merges catalog fields in so the gauge and stats keep working.
type Vehicle struct {
	// Instance identity
	ID      string  `json:"id"`
	ModelID string  `json:"modelId"`
	UserID  *string `json:"userId,omitempty"`
	// Nickname - defaults to model name; user can rename.
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`

	// Instance state
	CurrentPercent float64 `json:"currentPercent"`
	TargetPercent  float64 `json:"targetPercent"`

	// Catalog config merged in on read so consumers see the full VehicleSchema shape.
	ModelName           string   `json:"modelName"`
	CapacityKwh         float64  `json:"capacityKwh"`
	ChargerOutputW      float64  `json:"chargerOutputW"`
	ChargingEfficiency  float64  `json:"chargingEfficiency"`
	Time0to100Min       *int     `json:"time0to100Min,omitempty"`
	Time0to80Min        *int     `json:"time0to80Min,omitempty"`
	Time20to80Min       *int     `json:"time20to80Min,omitempty"`
	Time20to100Min      *int     `json:"time20to100Min,omitempty"`
	PackVoltageMaxV     *float64 `json:"packVoltageMaxV,omitempty"`
	PackCutoffCurrentMa *float64 `json:"packCutoffCurrentMa,omitempty"`
	RangeMinMi          float64    `json:"rangeMinMi"`
	RangeMaxMi          float64    `json:"rangeMaxMi"`
	TotalSessions        int        `json:"totalSessions"`
	TotalBatteryKwh      float64    `json:"totalBatteryKwh"`
	TotalWallKwh         float64    `json:"totalWallKwh"`
	TotalCo2Grams        float64    `json:"totalCo2Grams"`
	TotalCostPence       float64    `json:"totalCostPence"`
	MinSessionBatteryKwh float64    `json:"minSessionBatteryKwh"`
	MaxSessionBatteryKwh float64    `json:"maxSessionBatteryKwh"`
	LastSessionAt        *time.Time `json:"lastSessionAt,omitempty"`

	// Per-vehicle notification preferences. All default to true (opted in).
	NotifyChargeComplete      bool `json:"notifyChargeComplete"`
	NotifyChargerOffline      bool `json:"notifyChargerOffline"`
	NotifyMaintenanceOffline  bool `json:"notifyMaintenanceOffline"`
}
