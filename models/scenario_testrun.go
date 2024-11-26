package models

import "time"

type TestrunStatus int

const (
	Up TestrunStatus = iota
	Pending
	Down
	Unknown
)

func (t TestrunStatus) String() string {
	switch t {
	case Up:
		return "up"
	case Pending:
		return "pending"
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
	case "pending":
		return Pending
	}
	return Unknown
}

type ScenarioTestRun struct {
	ScenarioIterationId     string
	ScenarioId              string
	ScenarioLiveIterationId string
	CreatedAt               time.Time
	ExpiresAt               time.Time
	Status                  TestrunStatus
}

type ScenarioTestRunInput struct {
	ScenarioIterationId string
	LiveScenarioId      string
	ScenarioId          string
	Period              time.Duration
}
