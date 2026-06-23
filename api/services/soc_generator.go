package services

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/tasmota"
)

var ErrInvalidVehicleCapacity = errors.New("vehicle not found or has zero capacity")

// SOCGenerator calculates battery SOC from Tasmota energy readings and
// produces persistable snapshots.
type SOCGenerator struct{}

// NewSOCGenerator creates a new SOC generator.
func NewSOCGenerator() *SOCGenerator {
	return &SOCGenerator{}
}

// CalculateSOC returns the current battery SOC percentage and the updated
// LastBlendedKwh for persistence.
func (sg *SOCGenerator) CalculateSOC(session *models.ChargeSession, energy *tasmota.EnergyData, vehicle *models.Vehicle) (socPercent float64, lastBlendedKwh float64, err error) {
	if vehicle == nil || vehicle.CapacityKwh <= 0 {
		return 0, 0, ErrInvalidVehicleCapacity
	}

	result := CalculateProgress(session, energy, vehicle)
	return result.CurrentPercent, result.LastBlendedKwh, nil
}

// BuildSnapshot creates a persistable SOC snapshot from session ID and computed SOC.
func (sg *SOCGenerator) BuildSnapshot(sessionID string, socPercent float64) *models.SOCSnapshot {
	return &models.SOCSnapshot{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		SocPercent: socPercent,
		Timestamp:  time.Now(),
	}
}
