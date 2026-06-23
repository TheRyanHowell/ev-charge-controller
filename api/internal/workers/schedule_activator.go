package workers

import (
	"context"
	"time"

	"ev-charge-controller/api/models"
	"ev-charge-controller/api/services"
)

type ScheduleActivator struct {
	scheduleService *services.ScheduleService
	pollInterval    time.Duration
}

func NewScheduleActivator(scheduleService *services.ScheduleService) *ScheduleActivator {
	return &ScheduleActivator{
		scheduleService: scheduleService,
		pollInterval:    time.Duration(models.PollIntervalSec) * time.Second,
	}
}

func (w *ScheduleActivator) Start(ctx context.Context) {
	RunTickerWorker(ctx, w.pollInterval, "Schedule activator", w.scheduleService.CheckAndActivateAll)
}
