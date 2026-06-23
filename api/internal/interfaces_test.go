package internal_test

import (
	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/mqtt"
	"ev-charge-controller/api/repository"
)

var _ internal.ChargeSessionRepo = (*repository.ChargeSessionRepository)(nil)
var _ internal.VehicleRepo = (*repository.VehicleRepository)(nil)
var _ internal.ScheduleRepo = (*repository.ScheduleRepository)(nil)
var _ internal.PushSubscriptionRepo = (*repository.PushSubscriptionRepository)(nil)
var _ internal.UserRepo = (*repository.UserRepository)(nil)
var _ internal.RefreshTokenRepo = (*repository.RefreshTokenRepository)(nil)
var _ internal.PlugRepo = (*repository.PlugRepository)(nil)
var _ internal.PlugController = (*mqtt.Controller)(nil)
