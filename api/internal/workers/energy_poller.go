package workers

import (
	"context"
	"log/slog"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"
	"ev-charge-controller/api/tasmota"
)

const (
	powerThresholdPercent         = 0.5
	pendingSessionTimeout         = 60 * time.Second
	disconnectConsecutiveErrCount = 3
)

type EnergyPoller struct {
	chargeService          *services.ChargeSessionService
	pollInterval           time.Duration
	consecutiveTasmotaErrs int
}

func NewEnergyPoller(chargeService *services.ChargeSessionService) *EnergyPoller {
	return &EnergyPoller{
		chargeService: chargeService,
		pollInterval:  time.Duration(models.PollIntervalSec) * time.Second,
	}
}

func (p *EnergyPoller) Start(ctx context.Context) {
	RunTickerWorker(ctx, p.pollInterval, "Tasmota energy polling", p.tick)
}

// tick services every in-progress session on its own plug: pending sessions
// are activated once their plug starts drawing power, and charging sessions
// get their plug's latest cached energy reading persisted. Iterating all
// sessions (not just the most recent) is what keeps concurrent sessions on
// different plugs monitored.
func (p *EnergyPoller) tick(ctx context.Context) {
	checkPendingSessionTimeout(ctx, p.chargeService)

	sessions, err := p.chargeService.ListActiveSessions(ctx)
	if err != nil {
		slog.Error("Error listing active sessions for energy poll", "err", err)
		return
	}
	if len(sessions) == 0 {
		return
	}

	sawEnergy := false
	for i := range sessions {
		session := &sessions[i]
		if session.PlugID == nil {
			continue
		}
		energy := p.chargeService.LastEnergyForPlug(*session.PlugID)
		if energy == nil {
			slog.Warn("No cached energy data for session plug", "sessionID", session.ID, "plugID", *session.PlugID)
			continue
		}
		sawEnergy = true

		if session.Status == models.SessionStatusPending {
			activatePendingIfDrawing(ctx, p.chargeService, session, energy)
			continue
		}
		if energy.Power > 0 {
			p.chargeService.SaveEnergyReadings(ctx, *session.PlugID, energy)
		}
		slog.Debug("Tasmota energy", "plug_id", *session.PlugID, "total_kwh", energy.Total, "power_w", energy.Power)
	}

	if !sawEnergy {
		p.consecutiveTasmotaErrs++
		if p.consecutiveTasmotaErrs >= disconnectConsecutiveErrCount {
			slog.Warn("Consecutive Tasmota errors - checking for disconnected sessions", "count", p.consecutiveTasmotaErrs)
			p.chargeService.CheckAndCancelDisconnectedSession(ctx)
			p.consecutiveTasmotaErrs = 0
		}
		return
	}
	p.consecutiveTasmotaErrs = 0
}

func checkPendingSessionTimeout(ctx context.Context, service *services.ChargeSessionService) {
	if _, err := service.CancelPendingIfTimedOut(ctx, pendingSessionTimeout); err != nil {
		slog.Error("Error cancelling timed-out pending session", "err", err)
	}
}

// activatePendingIfDrawing activates a pending session once its own plug is
// drawing more than half the vehicle's rated charger output.
func activatePendingIfDrawing(ctx context.Context, service *services.ChargeSessionService, pendingSession *models.ChargeSession, energy *tasmota.EnergyData) {
	vehicle, err := service.FindVehicleByID(ctx, pendingSession.VehicleID)
	if err != nil || vehicle == nil {
		return
	}

	powerThreshold := vehicle.ChargerOutputW * powerThresholdPercent
	if energy.Power > powerThreshold {
		slog.Info("Pending session activated", "session_id", pendingSession.ID, "power_w", energy.Power, "threshold_w", powerThreshold)
		if _, err := service.ActivatePending(ctx, pendingSession.ID); err != nil {
			slog.Error("Error activating pending session", "session_id", pendingSession.ID, "err", err)
		}
	}
}
