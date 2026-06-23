package services

import (
	"context"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
)

// chartDataRepo combines the narrow interfaces needed by ChartDataService.
type chartDataRepo interface {
	internal.ChartSessionResolver
	internal.SnapshotReader
	internal.PowerReadingReader
}

// ChartDataService provides chart data (power readings, SOC snapshots) for sessions.
type ChartDataService struct {
	repo chartDataRepo
}

func NewChartDataService(repo chartDataRepo) *ChartDataService {
	return &ChartDataService{repo: repo}
}

// GetPowerReadings returns power readings for the resolved session.
func (s *ChartDataService) GetPowerReadings(ctx context.Context, sessionID, vehicleID string) ([]models.PowerReading, error) {
	session, err := s.repo.ResolveChartSession(ctx, sessionID, vehicleID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return []models.PowerReading{}, nil
	}
	return s.repo.GetPowerReadings(ctx, session.ID)
}

// GetSOCSnapshots returns SOC snapshots for the resolved session.
func (s *ChartDataService) GetSOCSnapshots(ctx context.Context, sessionID, vehicleID string) ([]models.SOCSnapshot, error) {
	session, err := s.repo.ResolveChartSession(ctx, sessionID, vehicleID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return []models.SOCSnapshot{}, nil
	}
	return s.repo.GetSOCSnapshots(ctx, session.ID)
}
