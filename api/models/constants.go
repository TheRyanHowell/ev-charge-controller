package models

import "time"

// Plug types.
const (
	// PlugTypeCharging is the default plug type for main EV charging plugs.
	PlugTypeCharging = "charging"
	// PlugTypeMaintenance is for 12V battery maintenance chargers that remain
	// always-on and do not participate in charge sessions or schedules.
	PlugTypeMaintenance = "maintenance"
)

// DefaultChargingEfficiency is the assumed wall-to-battery charging efficiency
// when a vehicle's configured value is zero or negative.
const DefaultChargingEfficiency = 0.8

// DefaultCostPerKwhPence is the fallback electricity rate (pence per kWh) used to
// seed a user's tariff when none is configured and DEFAULT_COST_PER_KWH is unset.
// Mirrors the historical NEXT_PUBLIC_COST_PER_KWH default on the UI.
const DefaultCostPerKwhPence = 24.83

// Polling intervals.
const (
	// PollIntervalSec is the interval in seconds for background polling goroutines
	// (Tasmota energy polling, auto-stop checking, schedule activation).
	PollIntervalSec = 5
)

// Percent boundaries.
const (
	// MaxPercent is the maximum valid percentage value for battery charge levels.
	MaxPercent = 100
	// DefaultCurrentPercent is the default starting current battery level
	// shown to users when no vehicle is selected.
	DefaultCurrentPercent = 20
	// DefaultTargetPercent is the default target battery level shown to users
	// when no vehicle is selected.
	DefaultTargetPercent = 80
)

// Energy resolution.
const (
	// EnergyResolutionDecimalPlaces is the number of decimal places used when
	// requesting Tasmota energy sensor resolution.
	EnergyResolutionDecimalPlaces = 4
)

// MQTT power confirmation.
var (
	// PowerConfirmationTimeout is how long to wait for a stat/POWER confirmation
	// after sending a power ON/OFF command to the plug.
	PowerConfirmationTimeout = 10 * time.Second
	// PowerConfirmationBestEffortTimeout is the shorter timeout used for best-effort
	// power-off after a failed confirmation (e.g., cancelling a pending session).
	PowerConfirmationBestEffortTimeout = 5 * time.Second
)

// Conditioning phase thresholds.
const (
	// ConditioningStopThresholdFraction is the fraction of charger output below which
	// a conditioning session is considered complete. CV tail current has tapered enough.
	ConditioningStopThresholdFraction = 0.10
)

// Two-stage (ready-by) charging thresholds.
const (
	// TwoStageHoldFraction is the fraction of a vehicle's target percent charged
	// during stage 1 before the plug is powered off and the session holds, ready
	// to resume in time to reach 100% of the target by the schedule's ready-by time.
	TwoStageHoldFraction = 0.8

	// MinTwoStageStageDurationMin is the minimum estimated stage 2 (hold->target)
	// charge duration, in minutes, for two-stage charging to be worthwhile. Below
	// this, the deferred top-up is shorter than the overhead of a relay power-cycle
	// and hold/resume state transition - the schedule falls back to a single-stage
	// charge straight to target instead. Only stage 2 is gated: a small stage 1
	// (current already close to the hold point) is not degenerate on its own,
	// since stage 2 may still be substantial.
	MinTwoStageStageDurationMin = 15
)

// LWT debounce and cooldown durations.
const (
	// LWTOfflineDebounce is how long to wait after an LWT Offline before marking
	// a plug unavailable. An Online within this window suppresses the transition.
	LWTOfflineDebounce = 60 * 1000000000 // 60 * time.Second in nanoseconds (avoids time import)
	// LWTOfflineCooldown is the minimum time between plug-unavailable notifications
	// for the same plug to suppress flap noise.
	LWTOfflineCooldown = 15 * 60 * 1000000000 // 15 minutes
)

// StopReason identifies why a charge session was stopped.
type StopReason string

const (
	// StopManual is set when the user manually stops the session.
	StopManual StopReason = "manual"
	// StopAutoComplete is set when the session auto-stops because the target was reached.
	StopAutoComplete StopReason = "auto_complete"
	// StopDisconnect is set when the plug goes offline mid-session.
	StopDisconnect StopReason = "disconnect"
)

// Server defaults.
const (
	// DefaultDBPath is the fallback database file path when DB_PATH is unset.
	DefaultDBPath = "./ev-charge.db"
	// DefaultServerPort is the fallback HTTP listen port when PORT is unset.
	DefaultServerPort = "8080"
)
