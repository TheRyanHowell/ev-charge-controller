package services

import (
	"sort"
	"time"

	"ev-charge-controller/api/models"
)

// CalculateSessionCost computes the time-weighted electricity cost for a session,
// splitting its wall-side energy across the tariff's off-peak windows by the
// timestamp at which each increment was actually drawn.
//
// startTotalKwh is the cumulative wall-side meter reading captured at session start
// (the baseline the first reading's delta is measured against). readings are the
// session's power readings, whose EnergyKwh is the cumulative wall-side meter total.
// Readings are sorted by timestamp defensively. Returns the total cost in pence and
// the wall-side kWh that was billed at an off-peak rate (on-peak = wallKwh - offPeak).
func CalculateSessionCost(startTotalKwh float64, readings []models.PowerReading, tariff models.TariffSettings) (totalPence, offPeakWallKwh float64) {
	if len(readings) == 0 {
		return 0, 0
	}

	sorted := make([]models.PowerReading, len(readings))
	copy(sorted, readings)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	prevKwh := startTotalKwh
	for _, reading := range sorted {
		delta := reading.EnergyKwh - prevKwh
		prevKwh = reading.EnergyKwh
		if delta <= 0 {
			// Skip non-positive deltas (sensor drift, baseline gaps, duplicates).
			continue
		}
		rate, isOffPeak := applicableRatePence(tariff, reading.Timestamp)
		totalPence += delta * rate
		if isOffPeak {
			offPeakWallKwh += delta
		}
	}
	return totalPence, offPeakWallKwh
}

// applicableRatePence returns the rate (pence/kWh) in effect at time t and whether
// it is an off-peak rate. The first off-peak window (in configured order) whose
// span contains t's local time-of-day wins; otherwise the base rate applies.
func applicableRatePence(tariff models.TariffSettings, t time.Time) (rate float64, offPeak bool) {
	minutes := t.Hour()*60 + t.Minute()
	for _, w := range tariff.OffPeakWindows {
		if withinWindow(minutes, w.Start, w.End) {
			return w.RatePence, true
		}
	}
	return tariff.BaseRatePence, false
}

// withinWindow reports whether minutes-of-day falls in [start, end). A window whose
// end is not after its start wraps past midnight (e.g. 23:30→05:30). A zero-length
// window (start == end) matches nothing. Invalid HH:MM strings match nothing.
func withinWindow(minutes int, startHHMM, endHHMM string) bool {
	startH, startM, err := parseHHMM(startHHMM)
	if err != nil {
		return false
	}
	endH, endM, err := parseHHMM(endHHMM)
	if err != nil {
		return false
	}
	start := startH*60 + startM
	end := endH*60 + endM
	if start == end {
		return false
	}
	if start < end {
		return minutes >= start && minutes < end
	}
	// Window wraps midnight.
	return minutes >= start || minutes < end
}
