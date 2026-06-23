package services

import (
	"log/slog"
	"math"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/tasmota"
)

// ProgressResult holds the computed charging progress for a session.
type ProgressResult struct {
	BlendedKwh     float64
	CurrentPercent float64
	HasTick        bool
	LastBlendedKwh float64 // updated monotonic clamp baseline for persistence
}

// CalculateCurrentPercent returns the battery SOC percentage for a session,
// using blended kWh with monotonic clamping.
func CalculateCurrentPercent(session *models.ChargeSession, energy *tasmota.EnergyData, vehicle *models.Vehicle) float64 {
	result := CalculateProgress(session, energy, vehicle)
	return result.CurrentPercent
}

// CalculateProgress computes the current charging progress for a session.
// It blends raw Tasmota energy with power-times-elapsed interpolation,
// applies monotonic clamping using session-scoped state, and calculates
// current percent. Returns the updated LastBlendedKwh for persistence.
func CalculateProgress(session *models.ChargeSession, energy *tasmota.EnergyData, vehicle *models.Vehicle) ProgressResult {
	if !hasEnergyInputs(session, energy, vehicle) {
		slog.Warn("[CALC] CalculateProgress: incomplete inputs, returning zero progress",
			"hasSession", session != nil,
			"hasEnergy", energy != nil,
			"hasVehicle", vehicle != nil,
			"hasStartTotalKwh", session != nil && session.StartTotalKwh != nil,
		)
		return ProgressResult{}
	}

	efficiency := vehicle.ChargingEfficiency
	if efficiency <= 0 {
		efficiency = models.DefaultChargingEfficiency
	}

	interpolationTime := session.CreatedAt
	if session.StartedAt != nil {
		interpolationTime = *session.StartedAt
	}
	blendedKwh, hasTick := CalcBlendedKwh(session.StartKwh, *session.StartTotalKwh, energy, interpolationTime, efficiency)
	preClampKwh := blendedKwh

	// Monotonic clamp using session-scoped baseline
	lastBlended := session.StartKwh
	if session.LastBlendedKwh != nil {
		lastBlended = *session.LastBlendedKwh
	}
	blendedKwh = clampMonotonic(blendedKwh, lastBlended)
	postClampKwh := blendedKwh
	blendedKwh = math.Max(session.StartKwh, math.Min(session.TargetKwh, blendedKwh))
	finalKwh := blendedKwh

	currentPercent := (finalKwh / vehicle.CapacityKwh) * 100

	slog.Debug("[CALC] CalculateProgress",
		"session", session.ID,
		"preClamp", preClampKwh,
		"postClamp", postClampKwh,
		"final", finalKwh,
		"currentPct", currentPercent,
		"targetPct", session.TargetPercent,
	)

	return ProgressResult{
		BlendedKwh:     finalKwh,
		CurrentPercent: currentPercent,
		HasTick:        hasTick,
		LastBlendedKwh: postClampKwh,
	}
}

// CurrentPercentFromBlended derives the battery SOC percent from the session's
// last persisted blended kWh (owned by the energy-poller worker). It is used on
// read paths when no live energy reading is available - e.g. the plug is idle
// between MQTT ticks, briefly reporting 0W, or the controller is not yet
// connected - so an active session keeps reporting its last-known progress
// instead of omitting currentPercent. Without this, a fresh page load shows the
// start percent until the next poll restores the live value.
//
// Returns ok=false when there is no persisted baseline or the vehicle capacity
// is invalid, in which case callers should leave currentPercent unset.
func CurrentPercentFromBlended(session *models.ChargeSession, vehicle *models.Vehicle) (float64, bool) {
	if session == nil || vehicle == nil || vehicle.CapacityKwh <= 0 || session.LastBlendedKwh == nil {
		return 0, false
	}
	blendedKwh := math.Max(session.StartKwh, math.Min(session.TargetKwh, *session.LastBlendedKwh))
	return (blendedKwh / vehicle.CapacityKwh) * 100, true
}

// CalculateEndPercent computes the final battery percent from wall-side energy.
// Unlike CalculateProgress (which uses blended kWh), this uses raw energy delta
// converted to battery-side via efficiency.
func CalculateEndPercent(session *models.ChargeSession, energy *tasmota.EnergyData, vehicle *models.Vehicle) float64 {
	if !hasEnergyInputs(session, energy, vehicle) {
		slog.Warn("[CALC] CalculateEndPercent: incomplete inputs, returning 0",
			"hasSession", session != nil,
			"hasEnergy", energy != nil,
			"hasVehicle", vehicle != nil,
			"hasStartTotalKwh", session != nil && session.StartTotalKwh != nil,
		)
		return 0
	}

	efficiency := vehicle.ChargingEfficiency
	if efficiency <= 0 {
		efficiency = models.DefaultChargingEfficiency
	}
	wallEnergyKwh := energy.Total - *session.StartTotalKwh
	endKwh := session.StartKwh + wallEnergyKwh*efficiency
	pct := (endKwh / vehicle.CapacityKwh) * 100

	return math.Max(0, math.Min(100, pct))
}

// hasEnergyInputs reports whether the inputs required for energy math are present
// and valid. The exported calculators dereference session.StartTotalKwh and divide
// by vehicle.CapacityKwh, so a missing baseline or non-positive capacity would
// otherwise panic or produce a non-finite percent.
func hasEnergyInputs(session *models.ChargeSession, energy *tasmota.EnergyData, vehicle *models.Vehicle) bool {
	return session != nil &&
		energy != nil &&
		vehicle != nil &&
		session.StartTotalKwh != nil &&
		vehicle.CapacityKwh > 0
}

// clampMonotonic ensures the value never regresses below the baseline.
// This is a pure function - state is managed by the caller.
func clampMonotonic(kwh, lastBlendedKwh float64) float64 {
	if kwh < lastBlendedKwh {
		slog.Debug("[CALC] clampMonotonic: regression prevented",
			"attempted", kwh,
			"clamped", lastBlendedKwh,
		)
		return lastBlendedKwh
	}
	return kwh
}
