// Package chargeestimate provides conservative charge-duration estimates for the
// carbon-aware scheduler. The estimates are deliberately over-stated (safe margin
// + CV penalty) so that latestStart = windowEnd − D is pulled earlier, guaranteeing
// the vehicle reaches target by the ready-by time.
package chargeestimate

import (
	"errors"
	"math"

	"ev-charge-controller/api/models"
)

// ErrNoEstimate is returned when the vehicle has insufficient spec data to estimate.
var ErrNoEstimate = errors.New("insufficient vehicle data for charge duration estimate")

// safetyMarginFactor adds 10% over-estimate to pull latestStart earlier.
const safetyMarginFactor = 1.10

// cvPenaltyFactor is applied when no CV spec time is available.
// CV phase is always slower than CC on a per-SOC-point basis; 2× is conservative.
const cvPenaltyFactor = 2.0

// EstimateMinutes returns the estimated number of minutes to charge a vehicle
// from currentPercent to targetPercent. The estimate is conservative (over-stated)
// to ensure the carbon-aware deadline guarantee is never missed.
//
// Returns (0, nil) when currentPercent >= targetPercent.
// Returns ErrNoEstimate when vehicle data is insufficient for any estimate.
func EstimateMinutes(v *models.Vehicle, currentPercent, targetPercent float64) (int, error) {
	if currentPercent >= targetPercent {
		return 0, nil
	}

	eff := v.ChargingEfficiency
	if eff <= 0 {
		eff = models.DefaultChargingEfficiency
	}

	t20to80, ok := deriveCCMinutes(v, eff)
	if !ok {
		return 0, ErrNoEstimate
	}

	t0to20 := derivePreCCMinutes(v, eff, t20to80)
	t80to100 := deriveCVMinutes(v, t20to80)

	var totalMin float64

	// Pre-CC phase: 0–20% SOC
	if currentPercent < 20.0 {
		end := math.Min(targetPercent, 20.0)
		frac := (end - currentPercent) / 20.0
		totalMin += t0to20 * frac
	}

	// CC phase: 20–80% SOC
	if currentPercent < 80.0 && targetPercent > 20.0 {
		start := math.Max(currentPercent, 20.0)
		end := math.Min(targetPercent, 80.0)
		if end > start {
			frac := (end - start) / 60.0
			totalMin += t20to80 * frac
		}
	}

	// CV phase: 80–100% SOC
	if targetPercent > 80.0 {
		start := math.Max(currentPercent, 80.0)
		frac := (targetPercent - start) / 20.0
		totalMin += t80to100 * frac
	}

	totalMin *= safetyMarginFactor
	return int(math.Ceil(totalMin)), nil
}

// ProjectPercent returns the SOC the vehicle will reach after availableMinutes of
// charging from currentPercent, using the same conservative estimate as EstimateMinutes.
// Used for shortfall notifications - the result is the pessimistic reachable SOC.
//
// Returns currentPercent if the vehicle data is insufficient.
func ProjectPercent(v *models.Vehicle, currentPercent float64, availableMinutes int) float64 {
	if availableMinutes <= 0 {
		return currentPercent
	}

	lo, hi := currentPercent, 100.0
	for i := 0; i < 64; i++ {
		mid := (lo + hi) / 2
		mins, err := EstimateMinutes(v, currentPercent, mid)
		if err != nil {
			return currentPercent
		}
		if mins <= availableMinutes {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// deriveCCMinutes returns the minutes required to charge through the 20–80% CC phase.
// Uses Time20to80Min directly, falling back to power-based estimation.
// Returns (0, false) when neither spec times nor power+capacity are available.
func deriveCCMinutes(v *models.Vehicle, eff float64) (float64, bool) {
	if v.Time20to80Min != nil && *v.Time20to80Min > 0 {
		return float64(*v.Time20to80Min), true
	}
	if v.ChargerOutputW > 0 && v.CapacityKwh > 0 {
		chargerKw := v.ChargerOutputW / 1000.0
		energy := v.CapacityKwh * 0.6
		return energy / (chargerKw * eff) * 60.0, true
	}
	return 0, false
}

// derivePreCCMinutes returns the minutes required to charge through the 0–20% pre-CC phase.
// Falls back to the CC rate (treating pre-CC as full-speed CC) when no spec is available.
func derivePreCCMinutes(v *models.Vehicle, eff, t20to80 float64) float64 {
	if v.Time0to80Min != nil && v.Time20to80Min != nil && *v.Time0to80Min > *v.Time20to80Min {
		return float64(*v.Time0to80Min - *v.Time20to80Min)
	}
	if v.Time0to100Min != nil && v.Time20to100Min != nil && *v.Time0to100Min > *v.Time20to100Min {
		return float64(*v.Time0to100Min - *v.Time20to100Min)
	}
	if v.ChargerOutputW > 0 && v.CapacityKwh > 0 {
		chargerKw := v.ChargerOutputW / 1000.0
		energy := v.CapacityKwh * 0.2
		return energy / (chargerKw * eff) * 60.0
	}
	// Fall back to CC rate for the 20% SOC span (conservative: same power as CC).
	return t20to80 / 60.0 * 20.0
}

// deriveCVMinutes returns the minutes required to charge through the 80–100% CV phase.
// Applies cvPenaltyFactor when no spec time is available, because CV is always slower.
func deriveCVMinutes(v *models.Vehicle, t20to80 float64) float64 {
	if v.Time20to100Min != nil && v.Time20to80Min != nil && *v.Time20to100Min > *v.Time20to80Min {
		return float64(*v.Time20to100Min - *v.Time20to80Min)
	}
	if v.Time0to100Min != nil && v.Time0to80Min != nil && *v.Time0to100Min > *v.Time0to80Min {
		return float64(*v.Time0to100Min - *v.Time0to80Min)
	}
	// Penalty fallback: CV per-SOC-point takes cvPenaltyFactor × CC per-SOC-point.
	// CC covers 60 SOC points in t20to80; CV covers 20 points.
	return (t20to80 / 60.0) * 20.0 * cvPenaltyFactor
}
