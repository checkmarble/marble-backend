package models

import "time"

type TestrunStatus int

const (
	Up TestrunStatus = iota
	Idle
	Down
	Unknown
)

func (t TestrunStatus) String() string {
	switch t {
	case Up:
		return "up"
	case Idle:
		return "idle"
	case Down:
		return "down"
	}
	return "unknown"
}

func ScenarioTestStatusFrom(s string) TestrunStatus {
	switch s {
	case "up":
		return Up
	case "down":
		return Down
	case "unknown":
		return Unknown
	}
	return Unknown
}

type ScenarioTestRun struct {
	ScenarioIterationId string
	ScenarioId          string
	Period              time.Duration
	Status              TestrunStatus
}

type ScenarioTestRunInput struct {
	ScenarioIterationId string
	ScenarioId          string
	Period              time.Duration
}
