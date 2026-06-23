package services

import (
	"math"
	"testing"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/tasmota"

	"github.com/stretchr/testify/assert"
)

func TestClampMonotonic(t *testing.T) {
	// clampMonotonic is a pure function - no state
	assert.Equal(t, 0.6, clampMonotonic(0.5, 0.6))
	assert.Equal(t, 0.7, clampMonotonic(0.7, 0.6))
	assert.Equal(t, 0.7, clampMonotonic(0.7, 0.7))
	assert.Equal(t, 0.65, clampMonotonic(0.65, 0.6))
}

func TestEnergyCalculator_CalculateProgress_GuardsIncompleteInputs(t *testing.T) {
	goodSession := &models.ChargeSession{StartKwh: 0.6, StartTotalKwh: ptrFloat64(1.0), TargetKwh: 1.6, CreatedAt: time.Now()}
	goodVehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	energy := &tasmota.EnergyData{Total: 1.1, Power: 45000.0}

	tests := map[string]struct {
		session *models.ChargeSession
		vehicle *models.Vehicle
		energy  *tasmota.EnergyData
	}{
		"nil session":       {nil, goodVehicle, energy},
		"nil vehicle":       {goodSession, nil, energy},
		"nil energy":        {goodSession, goodVehicle, nil},
		"nil StartTotalKwh": {&models.ChargeSession{StartKwh: 0.6, TargetKwh: 1.6, CreatedAt: time.Now()}, goodVehicle, energy},
		"zero capacity":     {goodSession, &models.Vehicle{ID: "rm1", CapacityKwh: 0, ChargingEfficiency: 0.8}, energy},
		"negative capacity": {goodSession, &models.Vehicle{ID: "rm1", CapacityKwh: -1, ChargingEfficiency: 0.8}, energy},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Must not panic and must not produce a non-finite percent.
			result := CalculateProgress(tc.session, tc.energy, tc.vehicle)
			assert.Equal(t, ProgressResult{}, result)
			assert.False(t, math.IsInf(result.CurrentPercent, 0))
			assert.False(t, math.IsNaN(result.CurrentPercent))

			assert.Equal(t, 0.0, CalculateCurrentPercent(tc.session, tc.energy, tc.vehicle))
			end := CalculateEndPercent(tc.session, tc.energy, tc.vehicle)
			assert.False(t, math.IsInf(end, 0))
			assert.Equal(t, 0.0, end)
		})
	}
}

func TestEnergyCalculator_CalculateProgress_Bounded(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078,
		StartTotalKwh: ptrFloat64(1.092),
		TargetKwh:     1.6,
		CreatedAt:     time.Now(),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	energy := &tasmota.EnergyData{Total: 1.100, Power: 45000.0}

	result := CalculateProgress(session, energy, vehicle)
	assert.GreaterOrEqual(t, result.BlendedKwh, session.StartKwh)
	assert.LessOrEqual(t, result.BlendedKwh, session.TargetKwh)
	assert.Greater(t, result.CurrentPercent, 0.0)
}

func TestEnergyCalculator_CalculateProgress_ZeroPower(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078,
		StartTotalKwh: ptrFloat64(1.092),
		TargetKwh:     1.6,
		CreatedAt:     time.Now(),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	energy := &tasmota.EnergyData{Total: 1.100, Power: 0}

	result := CalculateProgress(session, energy, vehicle)
	// With zero power, blendedKwh == startKwh, so percent == start percent
	assert.InDelta(t, 30.0, result.CurrentPercent, 0.01)
	assert.False(t, result.HasTick)
}

func TestEnergyCalculator_CalculateProgress_AtTarget(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078,
		StartTotalKwh: ptrFloat64(1.092),
		TargetKwh:     0.6078, // already at target
		CreatedAt:     time.Now(),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	energy := &tasmota.EnergyData{Total: 1.100, Power: 45000.0}

	result := CalculateProgress(session, energy, vehicle)
	assert.Equal(t, 0.6078, result.BlendedKwh)
}

func TestEnergyCalculator_CalculateProgress_DefaultEfficiency(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078,
		StartTotalKwh: ptrFloat64(1.092),
		TargetKwh:     1.6,
		CreatedAt:     time.Now(),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0} // zero efficiency
	energy := &tasmota.EnergyData{Total: 1.100, Power: 45000.0}

	result := CalculateProgress(session, energy, vehicle)
	assert.Greater(t, result.BlendedKwh, 0.6078)
}

func TestEnergyCalculator_CalculateProgress_NegativeRemaining(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078,
		StartTotalKwh: ptrFloat64(1.092),
		TargetKwh:     0.6078,
		CreatedAt:     time.Now(),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	energy := &tasmota.EnergyData{Total: 1.100, Power: 45000.0}

	result := CalculateProgress(session, energy, vehicle)
	// Should clamp to target
	assert.Equal(t, 0.6078, result.BlendedKwh)
}

func TestEnergyCalculator_CalculateEndPercent_Direct(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078,
		StartTotalKwh: ptrFloat64(1.092),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	energy := &tasmota.EnergyData{Total: 1.100}

	pct := CalculateEndPercent(session, energy, vehicle)
	// Wall delta: (1.100 - 1.092) = 0.008 kWh. Battery: 0.008*0.8 = 0.0064 kWh
	// End Kwh: 0.6078 + 0.0064 = 0.6142. Percent: 0.6142/2.026*100 = 30.32%
	assert.InDelta(t, 30.316, pct, 0.01)
}

func TestEnergyCalculator_CalculateEndPercent_Direct_DefaultEfficiency(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078,
		StartTotalKwh: ptrFloat64(1.092),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0}
	energy := &tasmota.EnergyData{Total: 1.100}

	pct := CalculateEndPercent(session, energy, vehicle)
	// Default efficiency is 0.8, same as above
	assert.InDelta(t, 30.316, pct, 0.01)
}

func TestEnergyCalculator_CalculateEndPercent_NegativeClamped(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078,
		StartTotalKwh: ptrFloat64(2.0),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	// energy.Total < startTotalKwh simulates sensor drift
	energy := &tasmota.EnergyData{Total: 1.0}

	pct := CalculateEndPercent(session, energy, vehicle)
	// Wall delta: (1.0 - 2.0) = -1.0 kWh. Battery: -1.0*0.8 = -0.80 kWh
	// End Kwh: 0.6078 - 0.80 = -0.1922. Percent: -9.49% → clamped to 0.0
	assert.Equal(t, 0.0, pct)
}

func TestEnergyCalculator_CalculateEndPercent_AboveCapacity(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      1.8,
		StartTotalKwh: ptrFloat64(1.0),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	// Large energy delta pushes above capacity
	energy := &tasmota.EnergyData{Total: 3.0}

	pct := CalculateEndPercent(session, energy, vehicle)
	// Wall delta: 2.0 kWh. Battery: 2.0*0.8 = 1.6 kWh
	// End Kwh: 1.8 + 1.6 = 3.4. Percent: 3.4/2.026*100 = 167.8% → clamped to 100.0
	assert.Equal(t, 100.0, pct)
}

func TestEnergyCalculator_CalculateEndPercent_CorrectUnits(t *testing.T) {
	// stateless calculator

	session := &models.ChargeSession{
		StartKwh:      0.6078,
		StartTotalKwh: ptrFloat64(1.0),
	}
	vehicle := &models.Vehicle{ID: "rm1", CapacityKwh: 2.026, ChargingEfficiency: 0.8}
	energy := &tasmota.EnergyData{Total: 1.5}

	// Wall delta: (1.5 - 1.0) = 0.5 kWh
	// Battery delta: 0.5 * 0.8 = 0.40 kWh
	// End Kwh: 0.6078 + 0.40 = 1.0078
	// Percent: 1.0078 / 2.026 * 100 = 49.74%
	pct := CalculateEndPercent(session, energy, vehicle)
	assert.InDelta(t, 49.74, pct, 0.1)
}

func TestEnergyCalculator_CalculateProgress_UsesNominalCapacity(t *testing.T) {
	// stateless calculator

	// RM2: CapacityKwh=5.46, Time20to80Min=150, ChargerOutputW=1200
	// This is the vehicle where the bug was confirmed in production.
	//
	// startKwh = 5.46 * 20 / 100 = 1.092 (always computed with CapacityKwh)
	//
	// If CalculateProgress uses a derived capacity (e.g. 5.0) instead of
	// CapacityKwh (5.46), then at zero energy delta:
	//   percent = 1.092 / 5.0 * 100 = 21.84% (WRONG - should be 20%)
	//
	// This is a regression test: startKwh always uses CapacityKwh, so
	// CalculateProgress must too.
	session := &models.ChargeSession{
		StartKwh:      1.092,
		StartTotalKwh: ptrFloat64(1.092),
		TargetKwh:     4.368,
		CreatedAt:     time.Now(),
	}
	vehicle := &models.Vehicle{
		ID:                 "rm2",
		CapacityKwh:        5.46,
		ChargingEfficiency: 0.8,
		Time20to80Min:      ptrInt(150),
		ChargerOutputW:     1200,
	}
	energy := &tasmota.EnergyData{Total: 1.092, Power: 45000.0} // zero delta

	result := CalculateProgress(session, energy, vehicle)

	// With zero energy delta, blendedKwh == startKwh, so percent == start percent.
	// BUG: EffectiveCapacityKwh(5.0) would give 1.092/5.0*100 = 21.84% instead of 20%.
	assert.InDelta(t, 20.0, result.CurrentPercent, 0.01)
	assert.InDelta(t, 1.092, result.BlendedKwh, 0.0001)

	// Also verify with a real energy delta that the percent scales correctly.
	// delta = 1.100 - 1.092 = 0.008, battery delta = 0.008 * 0.8 = 0.0064
	// blended = 1.092 + 0.0064 = 1.0984
	// percent = 1.0984 / 5.46 * 100 = 20.117%
	energy2 := &tasmota.EnergyData{Total: 1.100, Power: 45000.0}
	result2 := CalculateProgress(session, energy2, vehicle)
	expectedPct := (1.092 + 0.008*0.8) / vehicle.CapacityKwh * 100
	assert.InDelta(t, expectedPct, result2.CurrentPercent, 0.01)
}

func TestEnergyCalculator_CalculateEndPercent_UsesNominalCapacity(t *testing.T) {
	// stateless calculator

	// RM2: CapacityKwh=5.46, EffectiveCapacityKwh=5.0 (derived from T20to80)
	// At zero delta, endKwh == startKwh, so percent must equal start percent.
	// BUG: Using EffectiveCapacityKwh (5.0) would give 1.092/5.0*100 = 21.84% instead of 20%.
	session := &models.ChargeSession{
		StartKwh:      1.092,
		StartTotalKwh: ptrFloat64(1.092),
	}
	vehicle := &models.Vehicle{
		ID:                 "rm2",
		CapacityKwh:        5.46,
		ChargingEfficiency: 0.8,
		Time20to80Min:      ptrInt(150),
		ChargerOutputW:     1200,
	}
	energy := &tasmota.EnergyData{Total: 1.092} // zero delta

	pct := CalculateEndPercent(session, energy, vehicle)
	assert.InDelta(t, 20.0, pct, 0.01)

	// With energy delta: endKwh = 1.092 + 0.008*0.8 = 1.0984
	energy2 := &tasmota.EnergyData{Total: 1.100}
	pct2 := CalculateEndPercent(session, energy2, vehicle)
	expectedPct := (1.092 + 0.008*0.8) / vehicle.CapacityKwh * 100
	assert.InDelta(t, expectedPct, pct2, 0.01)
}

func TestEnergyCalculator_Consistency_CapacityScale(t *testing.T) {
	// stateless calculator

	// RM2: CapacityKwh=5.46, EffectiveCapacityKwh=5.0
	// All percent calculations must use CapacityKwh (nominal).
	// EffectiveCapacityKwh is only used for curve/ETA integration scale.
	vehicle := &models.Vehicle{
		ID:                 "rm2",
		CapacityKwh:        5.46,
		ChargingEfficiency: 0.8,
		Time20to80Min:      ptrInt(150),
		ChargerOutputW:     1200,
	}

	session := &models.ChargeSession{
		StartKwh:      1.092, // 5.46 * 20%
		StartTotalKwh: ptrFloat64(1.092),
		TargetKwh:     4.368, // 5.46 * 80%
		CreatedAt:     time.Now(),
	}
	energy := &tasmota.EnergyData{Total: 1.092, Power: 1510.0}

	// CalculateCurrentPercent and CalculateProgress must return the same percent
	cp := CalculateCurrentPercent(session, energy, vehicle)
	cp2 := CalculateProgress(session, energy, vehicle)
	assert.InDelta(t, cp, cp2.CurrentPercent, 0.01,
		"CalculateCurrentPercent and CalculateProgress must return the same percent")

	// At zero delta, percent must exactly match start percent
	assert.InDelta(t, 20.0, cp, 0.01,
		"Zero delta percent must equal start percent, not inflated by EffectiveCapacityKwh")

	// CalculateEndPercent must also use CapacityKwh
	endPct := CalculateEndPercent(session, energy, vehicle)
	assert.InDelta(t, cp, endPct, 0.01,
		"CalculateEndPercent must return the same percent as CalculateCurrentPercent")
}

// FuzzCalcBlendedKwh_NeverBelowStart verifies the monotonicity invariant:
// CalcBlendedKwh never returns a value below startKwh, regardless of energy
// inputs. This is the core property that prevents SoC from appearing to
// decrease during a session due to sensor noise or Tasmota counter resets.
func FuzzCalcBlendedKwh_NeverBelowStart(f *testing.F) {
	f.Add(0.5, 1.0, 0.8, 1200.0, 1.002)    // tick path: wallDelta * eff > epsilon
	f.Add(0.5, 1.0, 0.8, 1200.0, 1.0)      // no-tick path: wallDelta * eff <= epsilon
	f.Add(0.0, 0.0, 0.85, 800.0, 0.003)    // zero start, small tick
	f.Add(10.0, 20.0, 0.9, 0.0, 25.0)      // zero power → early return
	f.Add(5.0, 10.0, 0.95, 500.0, 9.999)   // total < startTotal (counter wrap / drift)
	f.Fuzz(func(t *testing.T, startKwh, startTotalKwh, efficiency, power, totalEnergy float64) {
		energy := &tasmota.EnergyData{Power: power, Total: totalEnergy}
		result, _ := CalcBlendedKwh(startKwh, startTotalKwh, energy, time.Now(), efficiency)
		if math.IsNaN(result) || math.IsInf(result, 0) {
			t.Fatalf("CalcBlendedKwh returned non-finite %v (startKwh=%v, startTotal=%v, total=%v, power=%v, eff=%v)",
				result, startKwh, startTotalKwh, totalEnergy, power, efficiency)
		}
		if result < startKwh {
			t.Fatalf("CalcBlendedKwh returned %v < startKwh %v - monotonicity violated (startTotal=%v, total=%v, power=%v, eff=%v)",
				result, startKwh, startTotalKwh, totalEnergy, power, efficiency)
		}
	})
}
