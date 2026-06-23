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
	powerThresholdPercent        = 0.5
	pendingSessionTimeout        = 60 * time.Second
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

func (p *EnergyPoller) tick(ctx context.Context) {
	checkPendingSessionTimeout(ctx, p.chargeService)

	energy, err := p.chargeService.GetEnergy(ctx)
	if err != nil || energy == nil {
		if err != nil {
			slog.Error("Error polling Tasmota energy", "err", err)
		} else {
			slog.Warn("Tasmota returned nil energy data")
		}
		p.consecutiveTasmotaErrs++
		if p.consecutiveTasmotaErrs >= disconnectConsecutiveErrCount {
			slog.Warn("Consecutive Tasmota errors - checking for disconnected session", "count", p.consecutiveTasmotaErrs)
			p.chargeService.CheckAndCancelDisconnectedSession(ctx)
			p.consecutiveTasmotaErrs = 0
		}
		return
	}

	p.consecutiveTasmotaErrs = 0

	checkPendingSessionActivation(ctx, p.chargeService, energy)

	if energy.Power > 0 {
		p.saveEnergyReadings(ctx, energy)
	}

	slog.Debug("Tasmota energy", "total_kwh", energy.Total, "power_w", energy.Power)
}

func (p *EnergyPoller) saveEnergyReadings(ctx context.Context, energy *tasmota.EnergyData) {
	p.chargeService.SaveEnergyReadings(ctx, energy)
}

func checkPendingSessionTimeout(ctx context.Context, service *services.ChargeSessionService) {
	if _, err := service.CancelPendingIfTimedOut(ctx, pendingSessionTimeout); err != nil {
		slog.Error("Error cancelling timed-out pending session", "err", err)
	}
}

func checkPendingSessionActivation(ctx context.Context, service *services.ChargeSessionService, energy *tasmota.EnergyData) {
	pendingSession, err := service.GetPending(ctx)
	if err != nil || pendingSession == nil {
		return
	}

	vehicle, err := service.FindVehicleByID(ctx, pendingSession.VehicleID)
	if err != nil || vehicle == nil {
		return
	}

	powerThreshold := vehicle.ChargerOutputW * powerThresholdPercent
	if energy.Power > powerThreshold {
		slog.Info("Pending session activated", "session_id", pendingSession.ID, "power_w", energy.Power, "threshold_w", powerThreshold)
		_, err := service.ActivatePending(ctx, pendingSession.ID)
		if err != nil {
			slog.Error("Error activating pending session", "session_id", pendingSession.ID, "err", err)
		}
	}
}
