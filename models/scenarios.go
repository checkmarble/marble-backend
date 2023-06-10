package models

import "time"

type Scenario struct {
	ID                string
	Name              string
	Description       string
	TriggerObjectType string
	CreatedAt         time.Time
	LiveVersionID     *string
	ScenarioType      ScenarioType
}

type ScenarioType int

const (
	RealTime ScenarioType = iota
	Scheduled
)

func (s ScenarioType) String() string {
	switch s {
	case RealTime:
		return "real_time"
	case Scheduled:
		return "scheduled"
	}
	return "unknown"
}

func ScenarioTypeFrom(s string) ScenarioType {
	switch s {
	case "real_time":
		return RealTime
	case "scheduled":
		return Scheduled
	}
	return RealTime
}

type CreateScenarioInput struct {
	Name              string
	Description       string
	TriggerObjectType string
	ScenarioType      ScenarioType
}

type UpdateScenarioInput struct {
	ID          string
	Name        *string
	Description *string
}

type ListScenariosFilters struct {
	OrgID        string
	ScenarioType *ScenarioType
	IsActive     *bool
}
