package services

import (
	"context"
	"testing"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
	"ev-charge-controller/api/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tariffCtx() context.Context {
	return internal.WithUserID(context.Background(), testUserID)
}

func newTariffService(t *testing.T) (*TariffService, context.Context) {
	t.Helper()
	db := setupServiceTestDB(t)
	return NewTariffService(repository.NewTariffRepository(db)), tariffCtx()
}

func TestTariffService_GetSettings_DefaultWhenUnset(t *testing.T) {
	svc, ctx := newTariffService(t)

	settings, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	assert.InDelta(t, models.DefaultCostPerKwhPence, settings.BaseRatePence, 1e-9)
	assert.Empty(t, settings.OffPeakWindows)
}

func TestTariffService_UpdateAndGet_Roundtrip(t *testing.T) {
	svc, ctx := newTariffService(t)

	in := &models.TariffSettings{
		BaseRatePence: 28.5,
		OffPeakWindows: []models.OffPeakWindow{
			{Start: "00:30", End: "04:30", RatePence: 7.0},
		},
	}
	require.NoError(t, svc.UpdateSettings(ctx, in))

	got, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	assert.InDelta(t, 28.5, got.BaseRatePence, 1e-9)
	require.Len(t, got.OffPeakWindows, 1)
	assert.Equal(t, "00:30", got.OffPeakWindows[0].Start)
	assert.Equal(t, "04:30", got.OffPeakWindows[0].End)
	assert.InDelta(t, 7.0, got.OffPeakWindows[0].RatePence, 1e-9)
}

func TestTariffService_Update_ReplacesWindowsWholesale(t *testing.T) {
	svc, ctx := newTariffService(t)

	require.NoError(t, svc.UpdateSettings(ctx, &models.TariffSettings{
		BaseRatePence: 25,
		OffPeakWindows: []models.OffPeakWindow{
			{Start: "00:30", End: "04:30", RatePence: 7.0},
			{Start: "13:00", End: "16:00", RatePence: 12.0},
		},
	}))
	require.NoError(t, svc.UpdateSettings(ctx, &models.TariffSettings{
		BaseRatePence:  26,
		OffPeakWindows: []models.OffPeakWindow{{Start: "01:00", End: "05:00", RatePence: 8.0}},
	}))

	got, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	require.Len(t, got.OffPeakWindows, 1)
	assert.Equal(t, "01:00", got.OffPeakWindows[0].Start)
}

func TestTariffService_UpdateValidation(t *testing.T) {
	svc, ctx := newTariffService(t)

	tests := []struct {
		name   string
		tariff *models.TariffSettings
	}{
		{"negative base rate", &models.TariffSettings{BaseRatePence: -1}},
		{"invalid start", &models.TariffSettings{BaseRatePence: 25, OffPeakWindows: []models.OffPeakWindow{{Start: "bad", End: "04:30", RatePence: 7}}}},
		{"invalid end", &models.TariffSettings{BaseRatePence: 25, OffPeakWindows: []models.OffPeakWindow{{Start: "00:30", End: "99:99", RatePence: 7}}}},
		{"equal start and end", &models.TariffSettings{BaseRatePence: 25, OffPeakWindows: []models.OffPeakWindow{{Start: "00:30", End: "00:30", RatePence: 7}}}},
		{"negative window rate", &models.TariffSettings{BaseRatePence: 25, OffPeakWindows: []models.OffPeakWindow{{Start: "00:30", End: "04:30", RatePence: -2}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.UpdateSettings(ctx, tt.tariff)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidTariff)
		})
	}
}

func TestTariffService_RequiresAuthenticatedUser(t *testing.T) {
	svc, _ := newTariffService(t)

	_, err := svc.GetSettings(context.Background())
	require.Error(t, err)

	err = svc.UpdateSettings(context.Background(), &models.TariffSettings{BaseRatePence: 25})
	require.Error(t, err)
}

func TestTariffService_EffectiveTariffForUser(t *testing.T) {
	svc, ctx := newTariffService(t)

	// Unset → default.
	eff, err := svc.EffectiveTariffForUser(ctx, testUserID)
	require.NoError(t, err)
	assert.InDelta(t, models.DefaultCostPerKwhPence, eff.BaseRatePence, 1e-9)

	// Configured → persisted values.
	require.NoError(t, svc.UpdateSettings(ctx, &models.TariffSettings{
		BaseRatePence:  31,
		OffPeakWindows: []models.OffPeakWindow{{Start: "23:30", End: "05:30", RatePence: 6.5}},
	}))
	eff, err = svc.EffectiveTariffForUser(ctx, testUserID)
	require.NoError(t, err)
	assert.InDelta(t, 31, eff.BaseRatePence, 1e-9)
	require.Len(t, eff.OffPeakWindows, 1)
}
