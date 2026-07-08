package models

type ScheduleType string

const (
	ScheduleTypeDaily       ScheduleType = "daily"
	ScheduleTypeCarbonAware ScheduleType = "carbon_aware"
)

type Schedule struct {
	ID                 string                `json:"id"`
	PlugID             *string               `json:"plugId,omitempty"`
	UserID             *string               `json:"userId,omitempty"`
	Type               ScheduleType          `json:"type"`
	Time               string                `json:"time"`                         // HH:MM - daily fire time
	WindowStart        *string               `json:"windowStart,omitempty"`        // HH:MM - carbon_aware earliest start
	WindowEnd          *string               `json:"windowEnd,omitempty"`          // HH:MM - carbon_aware ready-by time
	EstimatedStartTime *string               `json:"estimatedStartTime,omitempty"` // HH:MM - carbon_aware forecast-based start estimate; computed on read, not persisted
	ReadyBy            *string               `json:"readyBy,omitempty"`            // HH:MM - daily two-stage ready-by deadline
	TwoStage           bool                  `json:"twoStage,omitempty"`           // carbon_aware two-stage charging toggle
	EstimatedPlan      *TwoStagePlanEstimate `json:"estimatedPlan,omitempty"`      // daily or carbon_aware two-stage plan estimate; computed on read, not persisted
	TargetUnreachable  bool                  `json:"targetUnreachable,omitempty"`  // true when the estimated charge duration exceeds the time available before the deadline; computed on read, not persisted
	Enabled            bool                  `json:"enabled"`
}

// TwoStagePlanEstimate describes the currently estimated timeline for a
// carbon-aware two-stage schedule: when stage 1 (to the hold percent) is
// expected to run, when the hold begins, and when stage 2 (hold to target) is
// expected to run to finish by the ready-by deadline. Purely advisory -
// recomputed live on every read, not persisted, and can shift as the carbon
// forecast updates.
type TwoStagePlanEstimate struct {
	Stage1Start string `json:"stage1Start"` // HH:MM
	Stage1End   string `json:"stage1End"`   // HH:MM - hold begins here
	Stage2Start string `json:"stage2Start"` // HH:MM - hold ends here
	Stage2End   string `json:"stage2End"`   // HH:MM - target reached
}
