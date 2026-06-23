package models

type ScheduleType string

const (
	ScheduleTypeDaily       ScheduleType = "daily"
	ScheduleTypeCarbonAware ScheduleType = "carbon_aware"
)

type Schedule struct {
	ID          string       `json:"id"`
	PlugID      *string      `json:"plugId,omitempty"`
	UserID      *string      `json:"userId,omitempty"`
	Type        ScheduleType `json:"type"`
	Time        string       `json:"time"`                  // HH:MM - daily fire time
	WindowStart *string      `json:"windowStart,omitempty"` // HH:MM - carbon_aware earliest start
	WindowEnd   *string      `json:"windowEnd,omitempty"`   // HH:MM - carbon_aware ready-by time
	Enabled     bool         `json:"enabled"`
}
