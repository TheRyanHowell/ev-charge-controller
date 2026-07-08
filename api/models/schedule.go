package models

type ScheduleType string

const (
	ScheduleTypeDaily       ScheduleType = "daily"
	ScheduleTypeCarbonAware ScheduleType = "carbon_aware"
)

type Schedule struct {
	ID                 string       `json:"id"`
	PlugID             *string      `json:"plugId,omitempty"`
	UserID             *string      `json:"userId,omitempty"`
	Type               ScheduleType `json:"type"`
	Time               string       `json:"time"`                         // HH:MM - daily fire time
	WindowStart        *string      `json:"windowStart,omitempty"`        // HH:MM - carbon_aware earliest start
	WindowEnd          *string      `json:"windowEnd,omitempty"`          // HH:MM - carbon_aware ready-by time
	EstimatedStartTime *string      `json:"estimatedStartTime,omitempty"` // HH:MM - carbon_aware forecast-based start estimate; computed on read, not persisted
	ReadyBy            *string      `json:"readyBy,omitempty"`            // HH:MM - daily two-stage ready-by deadline
	Enabled            bool         `json:"enabled"`
}
