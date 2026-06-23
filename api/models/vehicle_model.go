package models

// VehicleModel is a catalog entry representing a specific motorcycle model.
// It holds configuration-only data shared across all user instances.
type VehicleModel struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	CapacityKwh         float64  `json:"capacityKwh"`
	ChargerOutputW      float64  `json:"chargerOutputW"`
	ChargingEfficiency  float64  `json:"chargingEfficiency"`
	Time0to100Min       *int     `json:"time0to100Min,omitempty"`
	Time0to80Min        *int     `json:"time0to80Min,omitempty"`
	Time20to80Min       *int     `json:"time20to80Min,omitempty"`
	Time20to100Min      *int     `json:"time20to100Min,omitempty"`
	PackVoltageMaxV     *float64 `json:"packVoltageMaxV,omitempty"`
	PackCutoffCurrentMa *float64 `json:"packCutoffCurrentMa,omitempty"`
	RangeMinMi          float64  `json:"rangeMinMi"`
	RangeMaxMi          float64  `json:"rangeMaxMi"`
}
