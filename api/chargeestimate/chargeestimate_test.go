package chargeestimate

import (
	"testing"

	"ev-charge-controller/api/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int { return &v }

// rm1Vehicle returns a vehicle with known spec times (Maeving RM1 style).
func rm1Vehicle() *models.Vehicle {
	return &models.Vehicle{
		CapacityKwh:        2.0,
		ChargerOutputW:     700,
		ChargingEfficiency: 0.8,
		Time0to80Min:       intPtr(175),
		Time20to80Min:      intPtr(100),
		Time20to100Min:     intPtr(175),
		RangeMinMi:         50,
		RangeMaxMi:         70,
	}
}

func TestEstimateMinutes_AlreadyAtTarget(t *testing.T) {
	v := rm1Vehicle()
	mins, err := EstimateMinutes(v, 80, 80)
	require.NoError(t, err)
	assert.Equal(t, 0, mins)
}

func TestEstimateMinutes_CurrentAboveTarget(t *testing.T) {
	v := rm1Vehicle()
	mins, err := EstimateMinutes(v, 85, 80)
	require.NoError(t, err)
	assert.Equal(t, 0, mins)
}

func TestEstimateMinutes_NoSpecsReturnsError(t *testing.T) {
	v := &models.Vehicle{
		CapacityKwh:        0,
		ChargerOutputW:     0,
		ChargingEfficiency: 0.8,
	}
	_, err := EstimateMinutes(v, 20, 80)
	assert.ErrorIs(t, err, ErrNoEstimate)
}

func TestEstimateMinutes_CCPhaseOnly(t *testing.T) {
	v := rm1Vehicle()
	mins, err := EstimateMinutes(v, 20, 80)
	require.NoError(t, err)
	// CC phase: frac=1.0, t20to80=100. With margin: ceil(100*1.1) = 111 (float64 1.1 is above 1.1)
	assert.Equal(t, 111, mins)
}

func TestEstimateMinutes_PartialCCPhase(t *testing.T) {
	v := rm1Vehicle()
	// 50→80%: frac=30/60=0.5, CC=50 min. With margin: ceil(50*1.1)=56 (float64 rounding up)
	mins, err := EstimateMinutes(v, 50, 80)
	require.NoError(t, err)
	assert.Equal(t, 56, mins)
}

func TestEstimateMinutes_CVPhaseWithSpec(t *testing.T) {
	v := rm1Vehicle()
	// 80→100%: CV time = Time20to100Min - Time20to80Min = 175-100 = 75 min
	// frac = (100-80)/20 = 1.0, total = 75. With margin: ceil(75*1.1) = ceil(82.5) = 83
	mins, err := EstimateMinutes(v, 80, 100)
	require.NoError(t, err)
	assert.Equal(t, 83, mins)
}

func TestEstimateMinutes_CVPhaseNoPenaltyFallback(t *testing.T) {
	v := &models.Vehicle{
		CapacityKwh:        2.0,
		ChargerOutputW:     700,
		ChargingEfficiency: 0.8,
		Time20to80Min:      intPtr(100),
		// No Time20to100Min - will use CVPenaltyFactor fallback
	}
	// CV penalty: t20to80/60*20*2 = 100/60*20*2 ≈ 66.67 min
	// With margin: ceil(66.67*1.1) = ceil(73.33) = 74
	mins, err := EstimateMinutes(v, 80, 100)
	require.NoError(t, err)
	// Should be > CC equivalent (which would be t20to80/60*20 ≈ 33.3 min)
	assert.Greater(t, mins, 50, "CV penalty path should produce more time than unpenalized")
	assert.Equal(t, 74, mins)
}

func TestEstimateMinutes_FullRange(t *testing.T) {
	v := rm1Vehicle()
	// 0→100%: pre-CC + CC + CV
	// pre-CC: t0to80-t20to80 = 175-100 = 75 min, frac=20/20=1 → 75
	// CC: 100*1 = 100
	// CV: 75*1 = 75
	// total = 250, with margin ceil(250*1.1) = ceil(275) = 275
	mins, err := EstimateMinutes(v, 0, 100)
	require.NoError(t, err)
	assert.Equal(t, 275, mins)
}

func TestEstimateMinutes_PowerFallbackCC(t *testing.T) {
	// Only capacity and charger output available - no spec times
	v := &models.Vehicle{
		CapacityKwh:        10.0,
		ChargerOutputW:     3300,
		ChargingEfficiency: 0.8,
	}
	// CC: energy=10*0.6=6kWh, power=3.3kW*0.8=2.64kW, time=6/2.64*60≈136.36min
	// With margin: ceil(136.36*1.1)=ceil(150.0)=150
	mins, err := EstimateMinutes(v, 20, 80)
	require.NoError(t, err)
	assert.Equal(t, 150, mins)
}

func TestEstimateMinutes_DefaultEfficiency(t *testing.T) {
	v := &models.Vehicle{
		CapacityKwh:        10.0,
		ChargerOutputW:     3300,
		ChargingEfficiency: 0, // zero → default 0.8
	}
	v2 := &models.Vehicle{
		CapacityKwh:        10.0,
		ChargerOutputW:     3300,
		ChargingEfficiency: 0.8,
	}
	mins1, err1 := EstimateMinutes(v, 20, 80)
	mins2, err2 := EstimateMinutes(v2, 20, 80)
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, mins2, mins1, "zero efficiency should fall back to default 0.8")
}

func TestEstimateMinutes_IsConservative(t *testing.T) {
	v := rm1Vehicle()
	// Estimate must be ≥ spec time (conservative). CC spec = 100 min, estimate ≥ 100.
	mins, err := EstimateMinutes(v, 20, 80)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, mins, 100, "estimate should be ≥ spec time")
	assert.LessOrEqual(t, mins, 120, "estimate should not exceed spec+20% (sanity bound)")
}

func TestEstimateMinutes_Monotonic(t *testing.T) {
	v := rm1Vehicle()
	prev := 0
	for target := 21; target <= 100; target += 5 {
		mins, err := EstimateMinutes(v, 20, float64(target))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, mins, prev, "estimate should be monotonically non-decreasing with target")
		prev = mins
	}
}

func TestProjectPercent_RoundTrip(t *testing.T) {
	v := rm1Vehicle()
	current := 20.0
	target := 80.0

	// Get time to reach 80%
	mins, err := EstimateMinutes(v, current, target)
	require.NoError(t, err)

	// Project what we reach in that time - should be close to 80%
	projected := ProjectPercent(v, current, mins)
	assert.InDelta(t, target, projected, 2.0, "projected should be within 2% of target when given exact time")
}

func TestProjectPercent_ZeroTime(t *testing.T) {
	v := rm1Vehicle()
	projected := ProjectPercent(v, 20, 0)
	assert.Equal(t, 20.0, projected)
}

func TestProjectPercent_NegativeTime(t *testing.T) {
	v := rm1Vehicle()
	projected := ProjectPercent(v, 20, -5)
	assert.Equal(t, 20.0, projected)
}

func TestProjectPercent_NoSpec(t *testing.T) {
	v := &models.Vehicle{ChargingEfficiency: 0.8}
	projected := ProjectPercent(v, 20, 60)
	assert.Equal(t, 20.0, projected, "should return currentPercent when no spec data")
}

func TestDerivePreCCMinutes_From0to80(t *testing.T) {
	v := &models.Vehicle{
		CapacityKwh:        2.0,
		ChargerOutputW:     700,
		ChargingEfficiency: 0.8,
		Time0to80Min:       intPtr(175),
		Time20to80Min:      intPtr(100),
	}
	got := derivePreCCMinutes(v, 0.8, 100)
	assert.Equal(t, 75.0, got, "175-100=75 min pre-CC")
}

func TestDerivePreCCMinutes_From0to100Fallback(t *testing.T) {
	v := &models.Vehicle{
		CapacityKwh:        2.0,
		ChargerOutputW:     700,
		ChargingEfficiency: 0.8,
		Time0to100Min:      intPtr(250),
		Time20to100Min:     intPtr(175),
	}
	got := derivePreCCMinutes(v, 0.8, 100)
	assert.Equal(t, 75.0, got, "250-175=75 min pre-CC from 0to100/20to100")
}

func TestDerivePreCCMinutes_PowerFallback(t *testing.T) {
	v := &models.Vehicle{
		CapacityKwh:        10.0,
		ChargerOutputW:     3300,
		ChargingEfficiency: 0.8,
	}
	// 10kWh * 0.2 / (3.3kW * 0.8) * 60 ≈ 45.45 min
	got := derivePreCCMinutes(v, 0.8, 100)
	assert.InDelta(t, 45.45, got, 0.5)
}
