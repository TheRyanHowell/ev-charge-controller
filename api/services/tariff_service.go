package services

import (
	"context"
	"errors"
	"fmt"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
)

// ErrInvalidTariff is returned when submitted tariff settings fail validation.
var ErrInvalidTariff = errors.New("invalid tariff settings")

// TariffService provides per-user electricity tariff settings. When a user has not
// configured a tariff, GetSettings returns a default seeded from the
// models.DefaultCostPerKwhPence constant.
type TariffService struct {
	repo internal.TariffRepo
}

// NewTariffService creates a TariffService.
func NewTariffService(repo internal.TariffRepo) *TariffService {
	return &TariffService{repo: repo}
}

// GetSettings returns the current user's tariff, or a default tariff (base rate
// only, no off-peak windows) when none has been configured.
func (s *TariffService) GetSettings(ctx context.Context) (*models.TariffSettings, error) {
	userID, ok := internal.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("tariff: user not authenticated")
	}
	settings, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return s.defaultSettings(), nil
	}
	if settings.OffPeakWindows == nil {
		settings.OffPeakWindows = []models.OffPeakWindow{}
	}
	return settings, nil
}

// UpdateSettings validates and persists the current user's tariff.
func (s *TariffService) UpdateSettings(ctx context.Context, settings *models.TariffSettings) error {
	userID, ok := internal.UserIDFromContext(ctx)
	if !ok {
		return errors.New("tariff: user not authenticated")
	}
	if err := validateTariff(settings); err != nil {
		return err
	}
	if settings.OffPeakWindows == nil {
		settings.OffPeakWindows = []models.OffPeakWindow{}
	}
	return s.repo.Upsert(ctx, userID, settings)
}

// EffectiveTariffForUser returns the tariff used to bill a specific user's session,
// falling back to the default tariff when the user has not configured one. Used by
// session completion (which may run without a user in context).
func (s *TariffService) EffectiveTariffForUser(ctx context.Context, userID string) (models.TariffSettings, error) {
	settings, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return models.TariffSettings{}, err
	}
	if settings == nil {
		return *s.defaultSettings(), nil
	}
	return *settings, nil
}

func (s *TariffService) defaultSettings() *models.TariffSettings {
	return &models.TariffSettings{
		BaseRatePence:  models.DefaultCostPerKwhPence,
		OffPeakWindows: []models.OffPeakWindow{},
	}
}

// validateTariff enforces a non-negative base rate and well-formed off-peak windows.
func validateTariff(settings *models.TariffSettings) error {
	if settings.BaseRatePence < 0 {
		return fmt.Errorf("%w: base rate must not be negative", ErrInvalidTariff)
	}
	for i, w := range settings.OffPeakWindows {
		if _, _, err := parseHHMM(w.Start); err != nil {
			return fmt.Errorf("%w: window %d start must be HH:MM", ErrInvalidTariff, i+1)
		}
		if _, _, err := parseHHMM(w.End); err != nil {
			return fmt.Errorf("%w: window %d end must be HH:MM", ErrInvalidTariff, i+1)
		}
		if w.Start == w.End {
			return fmt.Errorf("%w: window %d start and end must differ", ErrInvalidTariff, i+1)
		}
		if w.RatePence < 0 {
			return fmt.Errorf("%w: window %d rate must not be negative", ErrInvalidTariff, i+1)
		}
	}
	return nil
}
