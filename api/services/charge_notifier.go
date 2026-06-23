package services

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
)

// chargeNotificationTimeout is the maximum time allowed for sending a push notification.
const chargeNotificationTimeout = 5 * time.Second

// pushNotifier is the minimal interface for sending push notifications.
type pushNotifier interface {
	SendNotification(ctx context.Context, title, body string) error
}

// ChargeNotifier handles sending charge completion notifications.
//
// Notifications are sent on background goroutines so a slow push endpoint never
// blocks a session transition. Those goroutines are tracked on wg and rooted in
// baseCtx (the server context) so graceful shutdown can cancel in-flight sends
// and Wait drains them - they do not escape the shutdown sequence.
type ChargeNotifier struct {
	pushService pushNotifier
	vehicleRepo internal.VehicleReader
	plugRepo    internal.PlugRepo
	baseCtx     context.Context
	wg          sync.WaitGroup
}

// NewChargeNotifier creates a new ChargeNotifier. baseCtx is the parent context
// for notification goroutines; cancelling it (on shutdown) cancels in-flight sends.
func NewChargeNotifier(baseCtx context.Context, pushService *PushService, vehicleRepo internal.VehicleReader, plugRepo internal.PlugRepo) *ChargeNotifier {
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	return &ChargeNotifier{
		pushService: func() pushNotifier {
			if pushService == nil {
				return nil
			}
			return pushService
		}(),
		vehicleRepo: vehicleRepo,
		plugRepo:    plugRepo,
		baseCtx:     baseCtx,
	}
}

// Wait blocks until all in-flight notification goroutines have completed.
// Call during graceful shutdown after the base context is cancelled.
func (n *ChargeNotifier) Wait() {
	n.wg.Wait()
}

// NotifyChargeComplete sends a push notification when a charge session completes,
// gated on the vehicle's NotifyChargeComplete preference.
func (n *ChargeNotifier) NotifyChargeComplete(ctx context.Context, session *models.ChargeSession, actualEndPercent float64) {
	if n.pushService == nil {
		slog.Info("[ChargeNotifier] PushService not configured, skipping charge complete notification")
		return
	}

	vehicle, _ := n.vehicleRepo.FindByID(ctx, session.VehicleID)
	if vehicle != nil && !vehicle.NotifyChargeComplete {
		slog.Info("[ChargeNotifier] charge complete notification suppressed by vehicle preference", "vehicleID", vehicle.ID)
		return
	}

	name := ""
	if vehicle != nil {
		name = vehicle.Name
	}

	body := n.buildNotificationBody(name, actualEndPercent, vehicle)

	slog.Info("[ChargeNotifier] Sending charge complete notification", "sessionID", session.ID, "vehicle", name, "endPercent", actualEndPercent)

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()

		ctx, cancel := context.WithTimeout(n.baseCtx, chargeNotificationTimeout)
		defer cancel()

		if err := n.pushService.SendNotification(ctx, "Charge Complete", body); err != nil {
			slog.Error("[ChargeNotifier] Push notification error", "err", err)
		}
	}()
}

// NotifyPlugUnavailable sends a push notification when a plug goes offline.
// The notification is gated per-vehicle on the NotifyChargerOffline or
// NotifyMaintenanceOffline preference depending on plug type.
func (n *ChargeNotifier) NotifyPlugUnavailable(ctx context.Context, plug *models.Plug) {
	if n.pushService == nil {
		return
	}

	var title, body string

	if plug.VehicleID != nil && n.vehicleRepo != nil {
		vehicle, _ := n.vehicleRepo.FindByID(ctx, *plug.VehicleID)
		if vehicle != nil {
			if plug.Type == models.PlugTypeMaintenance {
				if !vehicle.NotifyMaintenanceOffline {
					slog.Info("[ChargeNotifier] maintenance offline notification suppressed by vehicle preference", "vehicleID", vehicle.ID)
					return
				}
				title = "12V Charger Offline"
				body = fmt.Sprintf("12V maintenance charger for %s is offline", vehicle.Name)
			} else {
				if !vehicle.NotifyChargerOffline {
					slog.Info("[ChargeNotifier] charger offline notification suppressed by vehicle preference", "vehicleID", vehicle.ID)
					return
				}
				title = "Charger Offline"
				body = fmt.Sprintf("Charger for %s is offline", vehicle.Name)
			}
		}
	}

	// Fallback when no vehicle is associated.
	if body == "" {
		if plug.Type == models.PlugTypeMaintenance {
			title = "12V Charger Offline"
			body = plug.Name + " (12V maintenance charger) is unavailable"
		} else {
			title = "Charger Offline"
			body = plug.Name + " is unavailable"
		}
	}

	slog.Info("[ChargeNotifier] Sending plug unavailable notification", "plug", plug.Name, "type", plug.Type)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		notifCtx, cancel := context.WithTimeout(n.baseCtx, chargeNotificationTimeout)
		defer cancel()
		if err := n.pushService.SendNotification(notifCtx, title, body); err != nil {
			slog.Error("[ChargeNotifier] Plug unavailable push error", "err", err)
		}
	}()
}

// NotifyShortfallProjected sends a push notification when a carbon-aware session
// is projected not to reach the target by the ready-by time.
func (n *ChargeNotifier) NotifyShortfallProjected(ctx context.Context, session *models.ChargeSession, projectedPercent, targetPercent float64, readyBy string) {
	if n.pushService == nil {
		slog.Info("[ChargeNotifier] PushService not configured, skipping shortfall notification")
		return
	}

	vehicle, _ := n.vehicleRepo.FindByID(ctx, session.VehicleID)
	name := ""
	if vehicle != nil {
		name = vehicle.Name
	}

	var body string
	if vehicle != nil {
		minRange := math.Round(vehicle.RangeMinMi * projectedPercent / 100)
		maxRange := math.Round(vehicle.RangeMaxMi * projectedPercent / 100)
		if minRange == maxRange && minRange > 0 {
			body = fmt.Sprintf("%s won't reach %.0f%% by %s - projected ~%.0f%% (~%.0fmi)", name, targetPercent, readyBy, projectedPercent, minRange)
		} else if minRange > 0 {
			body = fmt.Sprintf("%s won't reach %.0f%% by %s - projected ~%.0f%% (~%.0f-%.0fmi)", name, targetPercent, readyBy, projectedPercent, minRange, maxRange)
		} else {
			body = fmt.Sprintf("%s won't reach %.0f%% by %s - projected ~%.0f%%", name, targetPercent, readyBy, projectedPercent)
		}
	} else {
		body = fmt.Sprintf("Won't reach %.0f%% by %s - projected ~%.0f%%", targetPercent, readyBy, projectedPercent)
	}

	slog.Info("[ChargeNotifier] Sending shortfall notification", "sessionID", session.ID, "projectedPercent", projectedPercent, "readyBy", readyBy)

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		notifCtx, cancel := context.WithTimeout(n.baseCtx, chargeNotificationTimeout)
		defer cancel()
		if err := n.pushService.SendNotification(notifCtx, "Charging Shortfall", body); err != nil {
			slog.Error("[ChargeNotifier] Shortfall push error", "err", err)
		}
	}()
}

// buildNotificationBody constructs the notification body with range estimate.
// vehicle may be nil if the vehicle was not found; in that case a simpler message is returned.
func (n *ChargeNotifier) buildNotificationBody(vehicleName string, endPercent float64, vehicle *models.Vehicle) string {
	if vehicle == nil {
		return fmt.Sprintf("%s reached %.0f%%", vehicleName, endPercent)
	}

	minRange := math.Round(vehicle.RangeMinMi * endPercent / 100)
	maxRange := math.Round(vehicle.RangeMaxMi * endPercent / 100)

	if minRange == maxRange {
		return fmt.Sprintf("%s Charge Complete (%.0f%%, ~%.0fmi)", vehicleName, endPercent, minRange)
	}

	return fmt.Sprintf("%s Charge Complete (%.0f%%, ~%.0f-%.0fmi)", vehicleName, endPercent, minRange, maxRange)
}
