package services

import (
	"context"

	"ev-charge-controller/api/internal"
	"ev-charge-controller/api/models"
)

type HistoryQueryParams struct {
	VehicleID string
	Date      string
	Limit     int
	Offset    int
}

type HistorySessionResponse struct {
	models.ChargeSession
	TotalBatteryKwh              *float64 `json:"totalBatteryKwh,omitempty"`
	AvgCarbonIntensityGCo2PerKwh *float64 `json:"avgCarbonIntensityGCo2PerKwh,omitempty"`
}

// HistoryService provides charge session history with battery-side energy.
type HistoryService struct {
	sessionRepo      internal.SessionReader
	powerReadingStats internal.PowerReadingStats
}

func NewHistoryService(sessionRepo internal.SessionReader, powerReadingStats internal.PowerReadingStats) *HistoryService {
	return &HistoryService{
		sessionRepo:       sessionRepo,
		powerReadingStats: powerReadingStats,
	}
}

// GetHistory returns charge sessions with battery-side kWh and avg carbon intensity.
func (s *HistoryService) GetHistory(ctx context.Context, params HistoryQueryParams) ([]HistorySessionResponse, error) {
	sessions, err := s.fetchSessions(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, nil
	}

	sessionIDs := make([]string, len(sessions))
	for i, sess := range sessions {
		sessionIDs[i] = sess.ID
	}

	avgCI, err := s.powerReadingStats.GetAvgCarbonIntensityForSessions(ctx, sessionIDs)
	if err != nil {
		avgCI = map[string]*float64{}
	}

	return s.augmentSessions(sessions, avgCI), nil
}

func (s *HistoryService) fetchSessions(ctx context.Context, params HistoryQueryParams) ([]models.ChargeSession, error) {
	switch {
	case params.VehicleID != "" && params.Date != "":
		return s.sessionRepo.GetByVehicleAndDate(ctx, params.VehicleID, params.Date, params.Limit, params.Offset)
	case params.VehicleID != "":
		return s.sessionRepo.GetLatestByVehicle(ctx, params.VehicleID, params.Limit, params.Offset)
	case params.Date != "":
		return s.sessionRepo.GetByDate(ctx, params.Date, params.Limit, params.Offset)
	default:
		return s.sessionRepo.GetLatest(ctx, params.Limit, params.Offset)
	}
}

func (s *HistoryService) augmentSessions(sessions []models.ChargeSession, avgCI map[string]*float64) []HistorySessionResponse {
	responses := make([]HistorySessionResponse, 0, len(sessions))
	for _, sess := range sessions {
		resp := HistorySessionResponse{ChargeSession: sess}
		if sess.EndKwh != nil {
			batteryKwh := *sess.EndKwh - sess.StartKwh
			resp.TotalBatteryKwh = &batteryKwh
		} else if sess.LastBlendedKwh != nil && *sess.LastBlendedKwh > 0 {
			// Active/conditioning: energy added = absolute position minus start position.
			kwh := *sess.LastBlendedKwh - sess.StartKwh
			if kwh > 0 {
				resp.TotalBatteryKwh = &kwh
			}
		}
		resp.AvgCarbonIntensityGCo2PerKwh = avgCI[sess.ID]
		responses = append(responses, resp)
	}
	return responses
}
