package models

import (
	"context"
	"time"
)

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
	Id                      string
	ScenarioIterationId     string
	ScenarioId              string
	ScenarioLiveIterationId string
	OrganizationId          string
	CreatedAt               time.Time
	ExpiresAt               time.Time
	Status                  TestrunStatus
}

type ScenarioTestRunInput struct {
	ScenarioId         string
	PhantomIterationId string
	LiveScenarioId     string
	EndDate            time.Time
}

type OnCreateIndexesSuccess func(ctx context.Context) error
