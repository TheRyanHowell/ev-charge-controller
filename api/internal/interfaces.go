package internal

import (
	"context"
	"ev-charge-controller/api/carbonintensity"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/tasmota"
	"time"
)

// ChargeSessionRepo defines the methods needed by services for charge session data access.
type ChargeSessionRepo interface {
	GetActive(ctx context.Context) (*models.ChargeSession, error)
	GetActiveByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error)
	ListActive(ctx context.Context) ([]models.ChargeSession, error)
	GetPending(ctx context.Context) (*models.ChargeSession, error)
	GetPendingByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error)
	GetLastCompleted(ctx context.Context) (*models.ChargeSession, error)
	GetLastCompletedByVehicle(ctx context.Context, vehicleID string) (*models.ChargeSession, error)
	GetAll(ctx context.Context) ([]models.ChargeSession, error)
	GetAllByVehicle(ctx context.Context, vehicleID string) ([]models.ChargeSession, error)
	GetLatest(ctx context.Context, limit, offset int) ([]models.ChargeSession, error)
	GetLatestByVehicle(ctx context.Context, vehicleID string, limit, offset int) ([]models.ChargeSession, error)
	GetByDate(ctx context.Context, date string, limit, offset int) ([]models.ChargeSession, error)
	GetByVehicleAndDate(ctx context.Context, vehicleID, date string, limit, offset int) ([]models.ChargeSession, error)
	GetActiveByVehicle(ctx context.Context, vehicleID string) (*models.ChargeSession, error)
	FindByID(ctx context.Context, id string) (*models.ChargeSession, error)
	Create(ctx context.Context, session *models.ChargeSession) error
	ActivatePending(ctx context.Context, id string, startedAt time.Time) error
	UpdateStartTotalKwh(ctx context.Context, id string, startTotalKwh float64) error
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateEndedAt(ctx context.Context, id string, endedAt time.Time) error
	UpdateCancelData(ctx context.Context, id string, endedAt time.Time, endPercent *float64) error
	CancelPending(ctx context.Context, id string, endedAt time.Time) error
	UpdateEndWithStats(ctx context.Context, id string, endedAt time.Time, endKwh, endPercent float64, batteryKwh, wallKwh, co2Grams float64, avgCarbonIntensity *float64, costPence, offPeakKwh float64) error
	UpdateLastBlendedKwh(ctx context.Context, id string, lastBlendedKwh float64) error
	UpdateTarget(ctx context.Context, id string, targetPercent float64) error
	ResumeHolding(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	CreatePowerReading(ctx context.Context, reading *models.PowerReading) error
	CreateSOCSnapshot(ctx context.Context, snapshot *models.SOCSnapshot) error
	GetSOCSnapshots(ctx context.Context, sessionID string) ([]models.SOCSnapshot, error)
	GetPowerReadings(ctx context.Context, sessionID string) ([]models.PowerReading, error)
	ResolveChartSession(ctx context.Context, sessionID, vehicleID string) (*models.ChargeSession, error)
	GetSessionAggregates(ctx context.Context, vehicleID string, cutoff time.Time) (*models.SessionAggregates, error)
	GetDailyEnergy(ctx context.Context, vehicleID string, cutoff time.Time) ([]models.DailyEnergy, error)
}

// SessionReader provides read-only access to charge sessions.
type SessionReader interface {
	GetActive(ctx context.Context) (*models.ChargeSession, error)
	GetActiveByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error)
	ListActive(ctx context.Context) ([]models.ChargeSession, error)
	GetPending(ctx context.Context) (*models.ChargeSession, error)
	GetPendingByPlug(ctx context.Context, plugID string) (*models.ChargeSession, error)
	GetLastCompleted(ctx context.Context) (*models.ChargeSession, error)
	GetLastCompletedByVehicle(ctx context.Context, vehicleID string) (*models.ChargeSession, error)
	GetAll(ctx context.Context) ([]models.ChargeSession, error)
	GetAllByVehicle(ctx context.Context, vehicleID string) ([]models.ChargeSession, error)
	GetLatest(ctx context.Context, limit, offset int) ([]models.ChargeSession, error)
	GetLatestByVehicle(ctx context.Context, vehicleID string, limit, offset int) ([]models.ChargeSession, error)
	GetByDate(ctx context.Context, date string, limit, offset int) ([]models.ChargeSession, error)
	GetByVehicleAndDate(ctx context.Context, vehicleID, date string, limit, offset int) ([]models.ChargeSession, error)
	GetActiveByVehicle(ctx context.Context, vehicleID string) (*models.ChargeSession, error)
	FindByID(ctx context.Context, id string) (*models.ChargeSession, error)
	GetSOCSnapshots(ctx context.Context, sessionID string) ([]models.SOCSnapshot, error)
	GetPowerReadings(ctx context.Context, sessionID string) ([]models.PowerReading, error)
}

// SessionWriter provides write access to charge sessions.
type SessionWriter interface {
	Create(ctx context.Context, session *models.ChargeSession) error
	ActivatePending(ctx context.Context, id string, startedAt time.Time) error
	UpdateStartTotalKwh(ctx context.Context, id string, startTotalKwh float64) error
	UpdateStatus(ctx context.Context, id, status string) error
	UpdateEndedAt(ctx context.Context, id string, endedAt time.Time) error
	UpdateCancelData(ctx context.Context, id string, endedAt time.Time, endPercent *float64) error
	CancelPending(ctx context.Context, id string, endedAt time.Time) error
	UpdateEndWithStats(ctx context.Context, id string, endedAt time.Time, endKwh, endPercent float64, batteryKwh, wallKwh, co2Grams float64, avgCarbonIntensity *float64, costPence, offPeakKwh float64) error
	UpdateLastBlendedKwh(ctx context.Context, id string, lastBlendedKwh float64) error
	UpdateTarget(ctx context.Context, id string, targetPercent float64) error
	ResumeHolding(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

// SnapshotReader provides access to SOC snapshots.
type SnapshotReader interface {
	CreateSOCSnapshot(ctx context.Context, snapshot *models.SOCSnapshot) error
	GetSOCSnapshots(ctx context.Context, sessionID string) ([]models.SOCSnapshot, error)
	GetLastSOCSnapshot(ctx context.Context, sessionID string) (*models.SOCSnapshot, error)
}

// PowerReadingReader provides access to power readings.
type PowerReadingReader interface {
	CreatePowerReading(ctx context.Context, reading *models.PowerReading) error
	GetPowerReadings(ctx context.Context, sessionID string) ([]models.PowerReading, error)
	GetLastPowerReading(ctx context.Context, sessionID string) (*models.PowerReading, error)
}

// ChartSessionResolver provides session resolution for chart data.
type ChartSessionResolver interface {
	ResolveChartSession(ctx context.Context, sessionID, vehicleID string) (*models.ChargeSession, error)
}

// ChargeSessionServiceRepo provides the methods needed by ChargeSessionService.
// It composes reader/writer interfaces to provide a narrow, purpose-built interface
// that prevents the service from depending on unused repository operations.
type ChargeSessionServiceRepo interface {
	SessionReader
	SessionWriter
	SnapshotReader
	PowerReadingReader
	PowerReadingStats
}

// VehicleModelRepo provides read access to the global vehicle catalog.
type VehicleModelRepo interface {
	List(ctx context.Context) ([]models.VehicleModel, error)
	FindByID(ctx context.Context, id string) (*models.VehicleModel, error)
}

// VehicleRepo defines the methods needed by services for vehicle data access.
type VehicleRepo interface {
	FindByID(ctx context.Context, id string) (*models.Vehicle, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*models.Vehicle, error)
	List(ctx context.Context) ([]models.Vehicle, error)
	UpdatePercents(ctx context.Context, id string, currentPercent, targetPercent float64) error
	UpdateName(ctx context.Context, id, name, userID string) error
	UpdateNotificationPrefs(ctx context.Context, id, userID string, notifyChargeStarted, notifyChargeComplete, notifyChargerOffline, notifyMaintenanceOffline bool) error
	CreateInstance(ctx context.Context, vehicle *models.Vehicle) error
	DeleteInstance(ctx context.Context, id, userID string) error
	IncrementLifetimeStats(ctx context.Context, id string, batteryKwh, wallKwh, co2Grams, costPence float64, sessionAt time.Time) error
	DecrementLifetimeStats(ctx context.Context, id string, batteryKwh, wallKwh, co2Grams, costPence float64) error
}

// VehicleReader provides read-only access to vehicles.
type VehicleReader interface {
	FindByID(ctx context.Context, id string) (*models.Vehicle, error)
}

// ScheduleRepo defines the methods needed by services for schedule data access.
type ScheduleRepo interface {
	Get(ctx context.Context) (*models.Schedule, error)
	Upsert(ctx context.Context, schedule *models.Schedule) error
	GetByPlugID(ctx context.Context, plugID string) (*models.Schedule, error)
	UpsertByPlugID(ctx context.Context, schedule *models.Schedule) error
	ListAll(ctx context.Context) ([]models.Schedule, error)
}

// TariffReader reads a user's electricity tariff (base rate + off-peak windows).
type TariffReader interface {
	GetByUserID(ctx context.Context, userID string) (*models.TariffSettings, error)
}

// TariffRepo is the full tariff persistence surface.
type TariffRepo interface {
	TariffReader
	Upsert(ctx context.Context, userID string, settings *models.TariffSettings) error
}

// PushSubscriptionRepo defines the methods needed for push subscription data access.
type PushSubscriptionRepo interface {
	Upsert(ctx context.Context, sub *models.PushSubscription) error
	RemoveByEndpoint(ctx context.Context, endpoint string) error
	GetAll(ctx context.Context) ([]models.PushSubscription, error)
}

// UserRepo defines the methods needed for user data access.
type UserRepo interface {
	Create(ctx context.Context, user *models.User) error
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByID(ctx context.Context, id string) (*models.User, error)
}

// RefreshTokenRepo defines the methods needed for refresh token data access.
type RefreshTokenRepo interface {
	Create(ctx context.Context, token *models.RefreshToken) error
	FindByHash(ctx context.Context, hash string) (*models.RefreshToken, error)
	Revoke(ctx context.Context, id string) error
}

// PlugRepo defines methods for plug data access.
type PlugRepo interface {
	Create(ctx context.Context, plug *models.Plug) error
	FindByID(ctx context.Context, id string) (*models.Plug, error)
	FindByNamespaceAndSlug(ctx context.Context, namespace, slug string) (*models.Plug, error)
	ListNamespacesByUserID(ctx context.Context, userID string) ([]string, error)
	List(ctx context.Context, userID string) ([]models.Plug, error)
	Update(ctx context.Context, plug *models.Plug) error
	Delete(ctx context.Context, id, userID string) error
	SetOnline(ctx context.Context, plugID string, online bool) error
	UpdateLastOfflineNotifiedAt(ctx context.Context, plugID string) error
	SetInitialized(ctx context.Context, plugID string) error
	SetPowerState(ctx context.Context, plugID string, on bool) error
}

// PlugController provides power control and cached energy access for a plug.
type PlugController interface {
	SetPower(ctx context.Context, plugID string, on bool) error
	SetPowerAndWait(ctx context.Context, plugID string, on bool, timeout time.Duration) (bool, error)
	LastEnergy(plugID string) *tasmota.EnergyData
}

// CarbonIntensityFetcher fetches current grid carbon intensity.
// Implementations are expected to cache results.
type CarbonIntensityFetcher interface {
	GetCurrent(ctx context.Context) (*carbonintensity.CarbonIntensity, error)
}

// CarbonForecaster fetches 30-minute grid carbon intensity forecast buckets.
// Implementations are expected to cache the full 48h horizon per half-hour.
type CarbonForecaster interface {
	GetForecast(ctx context.Context, from, to time.Time) ([]carbonintensity.ForecastBucket, error)
}

// PowerReadingStats provides aggregate statistics over power readings.
type PowerReadingStats interface {
	GetAvgCarbonIntensityForSessions(ctx context.Context, sessionIDs []string) (map[string]*float64, error)
}
