package services

import (
	"math"
	"testing"
	"time"

	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
)

// reading builds a power reading at HH:MM (UTC) with the given cumulative wall kWh.
func reading(hour, minute int, cumulativeKwh float64) models.PowerReading {
	return models.PowerReading{
		Timestamp: time.Date(2026, 6, 21, hour, minute, 0, 0, time.UTC),
		EnergyKwh: cumulativeKwh,
	}
}

func TestCalculateSessionCost(t *testing.T) {
	offPeak := models.OffPeakWindow{Start: "00:30", End: "04:30", RatePence: 7.0}
	base := 24.0

	tests := []struct {
		name          string
		startTotalKwh float64
		readings      []models.PowerReading
		tariff        models.TariffSettings
		wantPence     float64
		wantOffPeak   float64
	}{
		{
			name:        "no readings is zero",
			readings:    nil,
			tariff:      models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{offPeak}},
			wantPence:   0,
			wantOffPeak: 0,
		},
		{
			name:          "no windows bills everything at base",
			startTotalKwh: 0,
			readings:      []models.PowerReading{reading(10, 0, 1), reading(10, 30, 3)},
			tariff:        models.TariffSettings{BaseRatePence: base},
			wantPence:     3 * base,
			wantOffPeak:   0,
		},
		{
			name:          "fully inside off-peak window",
			startTotalKwh: 0,
			readings:      []models.PowerReading{reading(1, 0, 2), reading(2, 0, 5)},
			tariff:        models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{offPeak}},
			wantPence:     5 * 7.0,
			wantOffPeak:   5,
		},
		{
			name:          "fully on-peak (outside window)",
			startTotalKwh: 0,
			readings:      []models.PowerReading{reading(12, 0, 2), reading(13, 0, 5)},
			tariff:        models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{offPeak}},
			wantPence:     5 * base,
			wantOffPeak:   0,
		},
		{
			name:          "spanning the boundary splits energy by reading time",
			startTotalKwh: 0,
			// 04:00 reading (+3 kWh, in window) then 05:00 reading (+2 kWh, out of window).
			readings:    []models.PowerReading{reading(4, 0, 3), reading(5, 0, 5)},
			tariff:      models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{offPeak}},
			wantPence:   3*7.0 + 2*base,
			wantOffPeak: 3,
		},
		{
			name:          "reading exactly at window end is on-peak",
			startTotalKwh: 0,
			// First reading at 04:30 (window end, exclusive) bills its +4 kWh at base.
			readings:    []models.PowerReading{reading(4, 30, 4)},
			tariff:      models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{offPeak}},
			wantPence:   4 * base,
			wantOffPeak: 0,
		},
		{
			name:          "midnight-wrapping window",
			startTotalKwh: 0,
			// 23:45 (+1, in window) and 00:15 (+2, in window) both off-peak.
			readings:    []models.PowerReading{reading(23, 45, 1), reading(0, 15, 3)},
			tariff:      models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{{Start: "23:30", End: "05:30", RatePence: 7.5}}},
			wantPence:   3 * 7.5,
			wantOffPeak: 3,
		},
		{
			name:          "multiple windows, first match wins",
			startTotalKwh: 0,
			readings:      []models.PowerReading{reading(2, 0, 4)},
			tariff: models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{
				{Start: "00:30", End: "04:30", RatePence: 7.0},
				{Start: "01:00", End: "03:00", RatePence: 5.0},
			}},
			wantPence:   4 * 7.0,
			wantOffPeak: 4,
		},
		{
			name:          "baseline offset from startTotalKwh",
			startTotalKwh: 10,
			readings:      []models.PowerReading{reading(1, 0, 12)}, // +2 kWh off-peak
			tariff:        models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{offPeak}},
			wantPence:     2 * 7.0,
			wantOffPeak:   2,
		},
		{
			name:          "non-positive deltas are skipped",
			startTotalKwh: 0,
			readings:      []models.PowerReading{reading(1, 0, 2), reading(1, 30, 2), reading(2, 0, 1), reading(2, 30, 4)},
			tariff:        models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{offPeak}},
			// +2 (off), +0 skip, -1 skip, +3 (off) → 5 kWh off-peak.
			wantPence:   5 * 7.0,
			wantOffPeak: 5,
		},
		{
			name:          "out-of-order readings are sorted",
			startTotalKwh: 0,
			readings:      []models.PowerReading{reading(5, 0, 5), reading(4, 0, 3)},
			tariff:        models.TariffSettings{BaseRatePence: base, OffPeakWindows: []models.OffPeakWindow{offPeak}},
			wantPence:     3*7.0 + 2*base,
			wantOffPeak:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPence, gotOffPeak := CalculateSessionCost(tt.startTotalKwh, tt.readings, tt.tariff)
			assert.InDelta(t, tt.wantPence, gotPence, 1e-9, "total pence")
			assert.InDelta(t, tt.wantOffPeak, gotOffPeak, 1e-9, "off-peak kWh")
		})
	}
}

// FuzzCalculateSessionCost_NonNegative verifies that CalculateSessionCost never
// produces a negative total pence value regardless of reading order or tariff
// config, as long as rates are non-negative. The partition property -
// offPeakWallKwh <= total wall kWh consumed - is also checked.
func FuzzCalculateSessionCost_NonNegative(f *testing.F) {
	f.Add(0.0, 24.0, 7.0, 1.0, 3.0)   // base and off-peak rates, two cumulative readings
	f.Add(0.0, 0.0, 0.0, 2.0, 5.0)    // zero rates: cost must be 0
	f.Add(5.0, 30.0, 10.0, 5.0, 5.0)  // non-zero start baseline (delta = 0)
	f.Fuzz(func(t *testing.T, startTotalKwh, baseRate, offPeakRate, reading1, reading2 float64) {
		if baseRate < 0 || offPeakRate < 0 {
			return // only test economically valid rates
		}
		tariff := models.TariffSettings{
			BaseRatePence: baseRate,
			OffPeakWindows: []models.OffPeakWindow{
				{Start: "00:30", End: "04:30", RatePence: offPeakRate},
			},
		}
		readings := []models.PowerReading{
			reading(1, 0, reading1),
			reading(2, 0, reading2),
		}
		totalPence, offPeakKwh := CalculateSessionCost(startTotalKwh, readings, tariff)
		if math.IsNaN(totalPence) || math.IsInf(totalPence, 0) {
			t.Fatalf("CalculateSessionCost: non-finite totalPence=%v", totalPence)
		}
		if totalPence < 0 {
			t.Fatalf("CalculateSessionCost: negative totalPence=%v (baseRate=%v, offPeakRate=%v)",
				totalPence, baseRate, offPeakRate)
		}
		if offPeakKwh < 0 {
			t.Fatalf("CalculateSessionCost: negative offPeakKwh=%v", offPeakKwh)
		}
		// offPeakKwh must not exceed total wall kWh consumed across all readings.
		var totalWallKwh float64
		prev := startTotalKwh
		for _, r := range readings {
			if delta := r.EnergyKwh - prev; delta > 0 {
				totalWallKwh += delta
			}
			prev = r.EnergyKwh
		}
		if offPeakKwh > totalWallKwh+1e-9 {
			t.Fatalf("CalculateSessionCost: offPeakKwh=%v > totalWallKwh=%v - partition violated",
				offPeakKwh, totalWallKwh)
		}
	})
}

func TestWithinWindow(t *testing.T) {
	tests := []struct {
		name       string
		minutes    int
		start, end string
		want       bool
	}{
		{"inside simple window", 120, "00:30", "04:30", true},
		{"before simple window", 10, "00:30", "04:30", false},
		{"at start is inclusive", 30, "00:30", "04:30", true},
		{"at end is exclusive", 270, "00:30", "04:30", false},
		{"wrap before midnight", 23*60 + 45, "23:30", "05:30", true},
		{"wrap after midnight", 15, "23:30", "05:30", true},
		{"wrap outside", 12 * 60, "23:30", "05:30", false},
		{"zero-length matches nothing", 30, "00:30", "00:30", false},
		{"invalid hhmm matches nothing", 30, "bad", "04:30", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, withinWindow(tt.minutes, tt.start, tt.end))
		})
	}
}
