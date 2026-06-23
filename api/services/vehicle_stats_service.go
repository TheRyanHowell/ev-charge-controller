package services

import (
	"context"
	"time"

	"ev-charge-controller/api/models"
)

// TimeRange represents a filtering window for statistics.
type TimeRange string

const (
	TimeRangeWeek     TimeRange = "week"
	TimeRangeMonth    TimeRange = "month"
	TimeRangeYear     TimeRange = "year"
	TimeRangeLifetime TimeRange = "lifetime"
)

// VehicleStatsQueryParams defines the query parameters for vehicle statistics.
type VehicleStatsQueryParams struct {
	VehicleID string
	Range     TimeRange
}

// DailyEnergy represents energy consumed on a specific day.
// Deprecated: use models.DailyEnergy instead. Kept for backward compatibility.
type DailyEnergy = models.DailyEnergy

// VehicleStats holds aggregate statistics for a vehicle over a time range.
type VehicleStats struct {
	TotalSessions        int           `json:"totalSessions"`
	TotalBatteryKwh      float64       `json:"totalBatteryKwh"`
	AvgSessionKwh        float64       `json:"avgSessionKwh"`
	AvgCarbonGCo2Kwh     *float64      `json:"avgCarbonGCo2PerKwh,omitempty"`
	TotalCo2Grams        float64       `json:"totalCo2Grams"`
	TotalCostPence       float64       `json:"totalCostPence"`
	AvgCostPence         float64       `json:"avgCostPence"`
	MinSessionBatteryKwh float64       `json:"minSessionBatteryKwh"`
	MaxSessionBatteryKwh float64       `json:"maxSessionBatteryKwh"`
	DailyEnergy          []DailyEnergy `json:"dailyEnergy"`
}

// VehicleSummaryStats holds minimal statistics for a single vehicle, used in list views.
type VehicleSummaryStats struct {
	VehicleID            string  `json:"vehicleId"`
	TotalSessions        int     `json:"totalSessions"`
	TotalBatteryKwh      float64 `json:"totalBatteryKwh"`
	AvgSessionWallKwh    float64 `json:"avgSessionWallKwh"`
	TotalCo2Grams        float64 `json:"totalCo2Grams"`
	TotalCostPence       float64 `json:"totalCostPence"`
	MinSessionBatteryKwh float64 `json:"minSessionBatteryKwh"`
	MaxSessionBatteryKwh float64 `json:"maxSessionBatteryKwh"`
	LastSessionAt        *string `json:"lastSessionAt,omitempty"`
}

type vehicleStatsRepo interface {
	FindByID(ctx context.Context, id string) (*models.Vehicle, error)
	List(ctx context.Context) ([]models.Vehicle, error)
	GetSessionAggregates(ctx context.Context, vehicleID string, cutoff time.Time) (*models.SessionAggregates, error)
	GetDailyEnergy(ctx context.Context, vehicleID string, cutoff time.Time) ([]models.DailyEnergy, error)
}

// statsRepoAdapter combines a vehicle reader and session aggregator to satisfy vehicleStatsRepo.
type statsRepoAdapter struct {
	vehicleRepo       vehicleReader
	sessionAggregator sessionAggregator
}

type vehicleReader interface {
	FindByID(ctx context.Context, id string) (*models.Vehicle, error)
	List(ctx context.Context) ([]models.Vehicle, error)
}

type sessionAggregator interface {
	GetSessionAggregates(ctx context.Context, vehicleID string, cutoff time.Time) (*models.SessionAggregates, error)
	GetDailyEnergy(ctx context.Context, vehicleID string, cutoff time.Time) ([]models.DailyEnergy, error)
}

func (a *statsRepoAdapter) FindByID(ctx context.Context, id string) (*models.Vehicle, error) {
	return a.vehicleRepo.FindByID(ctx, id)
}

func (a *statsRepoAdapter) List(ctx context.Context) ([]models.Vehicle, error) {
	return a.vehicleRepo.List(ctx)
}

func (a *statsRepoAdapter) GetSessionAggregates(ctx context.Context, vehicleID string, cutoff time.Time) (*models.SessionAggregates, error) {
	return a.sessionAggregator.GetSessionAggregates(ctx, vehicleID, cutoff)
}

func (a *statsRepoAdapter) GetDailyEnergy(ctx context.Context, vehicleID string, cutoff time.Time) ([]models.DailyEnergy, error) {
	return a.sessionAggregator.GetDailyEnergy(ctx, vehicleID, cutoff)
}

// VehicleStatsService computes aggregate statistics for a vehicle.
type VehicleStatsService struct {
	repo vehicleStatsRepo
}

func NewVehicleStatsService(repo vehicleStatsRepo) *VehicleStatsService {
	return &VehicleStatsService{repo: repo}
}

// NewVehicleStatsServiceWithRepos creates a VehicleStatsService from separate vehicle and session repos.
func NewVehicleStatsServiceWithRepos(vehicleR vehicleReader, sessionR sessionAggregator) *VehicleStatsService {
	return &VehicleStatsService{repo: &statsRepoAdapter{
		vehicleRepo:       vehicleR,
		sessionAggregator: sessionR,
	}}
}

// GetStats returns aggregate statistics for a vehicle within the given time range.
func (s *VehicleStatsService) GetStats(ctx context.Context, params VehicleStatsQueryParams) (*VehicleStats, error) {
	if params.Range == TimeRangeLifetime {
		return s.getLifetimeStats(ctx, params.VehicleID)
	}

	cutoff := getCutoff(time.Now(), params.Range)
	return s.getRangeStats(ctx, params.VehicleID, cutoff)
}

func (s *VehicleStatsService) getLifetimeStats(ctx context.Context, vehicleID string) (*VehicleStats, error) {
	vehicle, err := s.repo.FindByID(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	if vehicle == nil {
		return emptyStats(), nil
	}

	stats := mapVehicleToStats(vehicle)

	daily, err := s.repo.GetDailyEnergy(ctx, vehicleID, time.Time{})
	if err != nil {
		return nil, err
	}
	stats.DailyEnergy = daily
	return stats, nil
}

func (s *VehicleStatsService) getRangeStats(ctx context.Context, vehicleID string, cutoff time.Time) (*VehicleStats, error) {
	agg, err := s.repo.GetSessionAggregates(ctx, vehicleID, cutoff)
	if err != nil {
		return nil, err
	}
	if agg == nil || agg.TotalSessions == 0 {
		return emptyStats(), nil
	}

	stats := &VehicleStats{
		TotalSessions:        agg.TotalSessions,
		TotalBatteryKwh:      agg.TotalBatteryKwh,
		AvgSessionKwh:        agg.TotalBatteryKwh / float64(agg.TotalSessions),
		AvgCarbonGCo2Kwh:     agg.AvgCarbonGCo2Kwh,
		TotalCo2Grams:        agg.TotalCo2Grams,
		TotalCostPence:       agg.TotalCostPence,
		AvgCostPence:         agg.TotalCostPence / float64(agg.TotalSessions),
		MinSessionBatteryKwh: agg.MinSessionBatteryKwh,
		MaxSessionBatteryKwh: agg.MaxSessionBatteryKwh,
	}

	daily, err := s.repo.GetDailyEnergy(ctx, vehicleID, cutoff)
	if err != nil {
		return nil, err
	}
	stats.DailyEnergy = daily
	return stats, nil
}

func mapVehicleToStats(v *models.Vehicle) *VehicleStats {
	stats := &VehicleStats{
		TotalSessions:        v.TotalSessions,
		TotalBatteryKwh:      v.TotalBatteryKwh,
		TotalCo2Grams:        v.TotalCo2Grams,
		TotalCostPence:       v.TotalCostPence,
		MinSessionBatteryKwh: v.MinSessionBatteryKwh,
		MaxSessionBatteryKwh: v.MaxSessionBatteryKwh,
	}
	if v.TotalSessions > 0 {
		stats.AvgSessionKwh = v.TotalBatteryKwh / float64(v.TotalSessions)
		stats.AvgCostPence = v.TotalCostPence / float64(v.TotalSessions)
	}
	if v.TotalWallKwh > 0 && v.TotalCo2Grams > 0 {
		avg := v.TotalCo2Grams / v.TotalWallKwh
		stats.AvgCarbonGCo2Kwh = &avg
	}
	return stats
}

func getCutoff(now time.Time, tr TimeRange) time.Time {
	switch tr {
	case TimeRangeWeek:
		return now.AddDate(0, 0, -7)
	case TimeRangeMonth:
		return now.AddDate(0, -1, 0)
	case TimeRangeYear:
		return now.AddDate(-1, 0, 0)
	default:
		return time.Time{}
	}
}

func emptyStats() *VehicleStats {
	return &VehicleStats{
		DailyEnergy: []DailyEnergy{},
	}
}

// GetDailyEnergy returns daily energy data for a vehicle within the given time range.
func (s *VehicleStatsService) GetDailyEnergy(ctx context.Context, params VehicleStatsQueryParams) ([]DailyEnergy, error) {
	if params.Range == TimeRangeLifetime {
		return s.repo.GetDailyEnergy(ctx, params.VehicleID, time.Time{})
	}
	cutoff := getCutoff(time.Now(), params.Range)
	return s.repo.GetDailyEnergy(ctx, params.VehicleID, cutoff)
}

// GetAllVehiclesStats returns summary statistics for all vehicles.
func (s *VehicleStatsService) GetAllVehiclesStats(ctx context.Context) ([]VehicleSummaryStats, error) {
	vehicles, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]VehicleSummaryStats, 0, len(vehicles))
	for _, v := range vehicles {
		result = append(result, mapVehicleToSummary(&v))
	}
	return result, nil
}

func mapVehicleToSummary(v *models.Vehicle) VehicleSummaryStats {
	ss := VehicleSummaryStats{
		VehicleID:            v.ID,
		TotalSessions:        v.TotalSessions,
		TotalBatteryKwh:      v.TotalBatteryKwh,
		TotalCo2Grams:        v.TotalCo2Grams,
		TotalCostPence:       v.TotalCostPence,
		MinSessionBatteryKwh: v.MinSessionBatteryKwh,
		MaxSessionBatteryKwh: v.MaxSessionBatteryKwh,
	}
	if v.TotalSessions > 0 {
		ss.AvgSessionWallKwh = v.TotalWallKwh / float64(v.TotalSessions)
	}
	if v.LastSessionAt != nil {
		ts := v.LastSessionAt.Format(time.RFC3339)
		ss.LastSessionAt = &ts
	}
	return ss
}
