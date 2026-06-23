package workers

import (
	"context"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"
)

type AutoStopChecker struct {
	chargeService *services.ChargeSessionService
	pollInterval  time.Duration
}

func NewAutoStopChecker(chargeService *services.ChargeSessionService) *AutoStopChecker {
	return &AutoStopChecker{
		chargeService: chargeService,
		pollInterval:  time.Duration(models.PollIntervalSec) * time.Second,
	}
}

func (w *AutoStopChecker) Start(ctx context.Context) {
	RunTickerWorker(ctx, w.pollInterval, "Auto-stop checker", func(ctx context.Context) {
		w.chargeService.CheckAndAutoStopReachingSession(ctx)
		w.chargeService.CheckAndStopConditioningSession(ctx)
	})
}
