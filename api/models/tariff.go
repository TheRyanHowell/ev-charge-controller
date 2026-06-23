package models

import "time"

// OffPeakWindow is a local time-of-day window with its own electricity rate.
// Start and End are "HH:MM" (24h). End <= Start means the window wraps midnight
// (e.g. 23:30→05:30).
type OffPeakWindow struct {
	Start     string  `json:"start"`
	End       string  `json:"end"`
	RatePence float64 `json:"ratePence"`
}

// TariffSettings is a user's electricity tariff: a base (peak) rate plus zero or
// more off-peak windows. Cost is computed in pence per kWh of wall-side energy.
type TariffSettings struct {
	BaseRatePence  float64         `json:"baseRatePence"`
	OffPeakWindows []OffPeakWindow `json:"offPeakWindows"`
	UpdatedAt      *time.Time      `json:"updatedAt,omitempty"`
}
