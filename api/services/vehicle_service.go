package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
)

var (
	ErrVehicleNotFound            = errors.New("vehicle not found")
	ErrVehicleModelNotFound       = errors.New("vehicle model not found")
	ErrCurrentPercentOutOfRange   = errors.New("current battery level must be between 0 and 100")
	ErrTargetPercentOutOfRange    = errors.New("charge target must be between 0 and 100")
	ErrCurrentExceedsTarget       = errors.New("current battery level must be less than or equal to the charge target")
	ErrCurrentLockedDuringSession = errors.New("current battery level cannot be changed while a charge session is active")
	ErrDuplicateVehicleName       = errors.New("a vehicle with this name already exists")
	ErrNameRequired               = errors.New("vehicle name is required")
)

type VehicleService struct {
	repo          internal.VehicleRepo
	modelRepo     internal.VehicleModelRepo
	chargeSession internal.ChargeSessionRepo
	lock          sync.Locker
}

func NewVehicleService(
	repo internal.VehicleRepo,
	modelRepo internal.VehicleModelRepo,
	chargeSessionRepo internal.ChargeSessionRepo,
	lock sync.Locker,
) *VehicleService {
	return &VehicleService{
		repo:          repo,
		modelRepo:     modelRepo,
		chargeSession: chargeSessionRepo,
		lock:          lock,
	}
}

func (s *VehicleService) List(ctx context.Context) ([]models.Vehicle, error) {
	vehicles, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range vehicles {
		vehicles[i].CurrentPercent, vehicles[i].TargetPercent = s.resolvePercents(ctx, vehicles[i].ID)
	}
	return vehicles, nil
}

func (s *VehicleService) FindByID(ctx context.Context, id string) (*models.Vehicle, error) {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrVehicleNotFound
	}
	v.CurrentPercent, v.TargetPercent = s.resolvePercents(ctx, v.ID)
	return v, nil
}

// resolvePercents returns the current/target percent for a vehicle.
// Active charge session takes precedence; otherwise vehicle's stored values are used.
func (s *VehicleService) resolvePercents(ctx context.Context, vehicleID string) (currentPercent, targetPercent float64) {
	active, err := s.chargeSession.GetActiveByVehicle(ctx, vehicleID)
	if err == nil && active != nil {
		return active.StartPercent, active.TargetPercent
	}

	// No active session - use vehicle's stored values
	v, err := s.repo.FindByID(ctx, vehicleID)
	if err != nil {
		slog.Warn("failed to lookup vehicle percents, using defaults", "vehicle_id", vehicleID, "err", err)
		return models.DefaultCurrentPercent, models.DefaultTargetPercent
	}
	if v != nil {
		return v.CurrentPercent, v.TargetPercent
	}
	return models.DefaultCurrentPercent, models.DefaultTargetPercent
}

// UpdatePercents performs a partial update of a vehicle's stored current and/or target percent.
// A nil pointer means "leave this field unchanged." If an active charge session exists,
// updating currentPercent is rejected; targetPercent may still be updated.
func (s *VehicleService) UpdatePercents(ctx context.Context, id string, currentPercent, targetPercent *float64) error {
	// Serialise the whole check-then-act against session lifecycle transitions
	// and against other concurrent percent writes, so the active-session check
	// and the write cannot interleave and concurrent writes are deterministic.
	s.lock.Lock()
	defer s.lock.Unlock()

	if currentPercent != nil {
		active, err := s.chargeSession.GetActiveByVehicle(ctx, id)
		if err != nil {
			return err
		}
		if active != nil {
			return ErrCurrentLockedDuringSession
		}
	}

	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v == nil {
		return ErrVehicleNotFound
	}

	// Resolve effective values: use provided value or fall back to what's stored.
	effective := struct{ current, target float64 }{v.CurrentPercent, v.TargetPercent}
	if currentPercent != nil {
		effective.current = *currentPercent
	}
	if targetPercent != nil {
		effective.target = *targetPercent
	}

	if effective.current < 0 || effective.current > models.MaxPercent {
		return ErrCurrentPercentOutOfRange
	}
	if effective.target < 0 || effective.target > models.MaxPercent {
		return ErrTargetPercentOutOfRange
	}
	if effective.current > effective.target {
		return ErrCurrentExceedsTarget
	}
	return s.repo.UpdatePercents(ctx, id, effective.current, effective.target)
}

// ListModels returns the global vehicle catalog.
func (s *VehicleService) ListModels(ctx context.Context) ([]models.VehicleModel, error) {
	return s.modelRepo.List(ctx)
}

// AddVehicle creates a per-user instance from a catalog model.
// name defaults to the model's name when empty.
func (s *VehicleService) AddVehicle(ctx context.Context, userID, modelID, name string) (*models.Vehicle, error) {
	m, err := s.modelRepo.FindByID(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrVehicleModelNotFound
	}
	if name == "" {
		name = m.Name
	}
	v := &models.Vehicle{
		ModelID:        modelID,
		UserID:         &userID,
		Name:           name,
		CurrentPercent: models.DefaultCurrentPercent,
		TargetPercent:  models.DefaultTargetPercent,
	}
	if err := s.repo.CreateInstance(ctx, v); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicateVehicleName
		}
		return nil, err
	}
	// Merge catalog config for the response.
	v.CapacityKwh = m.CapacityKwh
	v.ChargerOutputW = m.ChargerOutputW
	v.ChargingEfficiency = m.ChargingEfficiency
	v.Time0to100Min = m.Time0to100Min
	v.Time0to80Min = m.Time0to80Min
	v.Time20to80Min = m.Time20to80Min
	v.Time20to100Min = m.Time20to100Min
	v.PackVoltageMaxV = m.PackVoltageMaxV
	v.PackCutoffCurrentMa = m.PackCutoffCurrentMa
	v.RangeMinMi = m.RangeMinMi
	v.RangeMaxMi = m.RangeMaxMi
	return v, nil
}

// UpdateName updates the user-visible nickname of a vehicle instance.
func (s *VehicleService) UpdateName(ctx context.Context, id, name string) error {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v == nil {
		return ErrVehicleNotFound
	}
	if name == "" {
		return ErrNameRequired
	}
	err = s.repo.UpdateName(ctx, id, name, *v.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrDuplicateVehicleName
		}
		return err
	}
	return nil
}

// UpdateNotificationPrefs updates the per-vehicle push notification preferences.
func (s *VehicleService) UpdateNotificationPrefs(ctx context.Context, userID, id string, notifyChargeComplete, notifyChargerOffline, notifyMaintenanceOffline bool) error {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v == nil || (v.UserID != nil && *v.UserID != userID) {
		return ErrVehicleNotFound
	}
	return s.repo.UpdateNotificationPrefs(ctx, id, userID, notifyChargeComplete, notifyChargerOffline, notifyMaintenanceOffline)
}

// DeleteVehicle removes a user's vehicle instance.
func (s *VehicleService) DeleteVehicle(ctx context.Context, userID, id string) error {
	v, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if v == nil || (v.UserID != nil && *v.UserID != userID) {
		return ErrVehicleNotFound
	}
	return s.repo.DeleteInstance(ctx, id, userID)
}
