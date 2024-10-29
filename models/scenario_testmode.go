package models

import "time"

type TestrunStatus int

const (
	Up TestrunStatus = iota
	Down
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
