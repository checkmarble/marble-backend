package models

import "time"

type TestrunStatus int

const (
	Up TestrunStatus = iota
	Down
	Unknown
)

func (t TestrunStatus) String() string {
	switch t {
	case Up:
		return "Up"
	case Down:
		return "Down"
	}
	return "unknown"
}

func ScenarioTestStatusFrom(s string) TestrunStatus {
	switch s {
	case "Up":
		return Up
	case "Down":
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
