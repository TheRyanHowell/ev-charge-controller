package models

import (
	"time"
)

// Session status constants.
const (
	SessionStatusIdle         = "idle"
	SessionStatusPending      = "pending"
	SessionStatusActive       = "active"
	SessionStatusConditioning = "conditioning"
	SessionStatusHolding      = "holding"
	SessionStatusCompleted    = "completed"
	SessionStatusCancelled    = "cancelled"
)

// ActiveSessionStatuses are the statuses considered "in progress": a vehicle in
// any of these has a live session. Used by both the global and per-vehicle
// active-session lookups so they stay symmetric.
var ActiveSessionStatuses = []string{
	SessionStatusActive,
	SessionStatusPending,
	SessionStatusConditioning,
	SessionStatusHolding,
}

type ChargeSession struct {
	ID             string     `json:"id"`
	VehicleID      string     `json:"vehicleId"`
	UserID         *string    `json:"userId,omitempty"`
	PlugID         *string    `json:"plugId,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
	StartKwh       float64    `json:"startKwh"`
	EndKwh         *float64   `json:"endKwh,omitempty"`
	TargetKwh      float64    `json:"targetKwh"`
	StartPercent   float64    `json:"startPercent"`
	EndPercent     *float64   `json:"endPercent,omitempty"`
	TargetPercent  float64    `json:"targetPercent"`
	Status         string     `json:"status"`
	StartTotalKwh    *float64 `json:"startTotalKwh,omitempty"`
	LastBlendedKwh   *float64 `json:"lastBlendedKwh,omitempty"`
	BatteryKwh       *float64 `json:"batteryKwh,omitempty"`
	WallKwh          *float64 `json:"wallKwh,omitempty"`
	AvgCarbonIntensity *float64 `json:"avgCarbonIntensity,omitempty"`
	Co2Grams         *float64 `json:"co2Grams,omitempty"`
	CostPence        *float64 `json:"costPence,omitempty"`
	OffPeakKwh       *float64 `json:"offPeakKwh,omitempty"`
	// HoldPercent is the intermediate stage-1 target for two-stage (ready-by)
	// charging. Set at session creation, cleared back to nil on resume - this
	// distinguishes "still charging toward the hold point" from "resumed,
	// charging toward the real target" without an extra column.
	HoldPercent *float64 `json:"holdPercent,omitempty"`
	// ReadyByTime is the HH:MM deadline carried from the originating schedule,
	// used to compute when to resume from the holding phase.
	ReadyByTime *string `json:"readyByTime,omitempty"`
	// CarbonAwareHold marks a two-stage session as originating from a
	// carbon-aware schedule: the resume decision consults the carbon forecast
	// (via findBalancedStart) instead of the plain deadline guard daily
	// two-stage sessions use.
	CarbonAwareHold bool `json:"carbonAwareHold,omitempty"`
}

// ChargeSessionView extends ChargeSession with computed fields for API responses.
type ChargeSessionView struct {
	ChargeSession
	PowerDraw      *float64 `json:"powerDraw,omitempty"`
	CurrentPercent *float64 `json:"currentPercent,omitempty"`
	EnergyAddedKwh *float64 `json:"energyAddedKwh,omitempty"`
	Voltage        *float64 `json:"voltage,omitempty"`
	Current        *float64 `json:"current,omitempty"`
}

type PowerReading struct {
	ID                       string    `json:"id"`
	SessionID                string    `json:"sessionId"`
	Timestamp                time.Time `json:"timestamp"`
	Voltage                  float64   `json:"voltage"`
	Current                  float64   `json:"current"`
	Power                    float64   `json:"power"`
	EnergyKwh                float64   `json:"energyKwh"`
	CarbonIntensityGCo2PerKwh *float64 `json:"carbonIntensityGCo2PerKwh,omitempty"`
}

type SOCSnapshot struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"sessionId"`
	Timestamp  time.Time `json:"timestamp"`
	SocPercent float64   `json:"socPercent"`
}

// SessionAggregates holds pre-computed aggregate statistics for a set of sessions.
type SessionAggregates struct {
	TotalSessions        int      `json:"totalSessions"`
	TotalBatteryKwh      float64  `json:"totalBatteryKwh"`
	TotalWallKwh         float64  `json:"totalWallKwh"`
	TotalCo2Grams        float64  `json:"totalCo2Grams"`
	TotalCostPence       float64  `json:"totalCostPence"`
	AvgCarbonGCo2Kwh     *float64 `json:"avgCarbonGCo2Kwh,omitempty"`
	MinSessionBatteryKwh float64  `json:"minSessionBatteryKwh"`
	MaxSessionBatteryKwh float64  `json:"maxSessionBatteryKwh"`
}

// DailyEnergy represents energy consumed on a specific day.
type DailyEnergy struct {
	Date                       string   `json:"date"`
	BatteryKwh                 float64  `json:"batteryKwh"`
	SessionCount               int      `json:"sessionCount"`
	Co2Grams                   float64  `json:"co2Grams"`
	AvgCarbonIntensityGCo2PerKwh *float64 `json:"avgCarbonIntensityGCo2PerKwh,omitempty"`
}
