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
	// Summarized indicates whether this test run results were summarized by the background worker.
	// By the nature of how we summarize results (incrementally for the missing period), we cannot stop
	// processing results when the test run ends), or we would miss results, so we continue processing until
	// we reach a watermark later than the test run end date, then we set this boolean to true.
	Summarized bool
	// This is used a an idempotency key to make concurrent runs of the test run summary job do not produce incorrect results.
	// Please do not update outside of this.
	UpdatedAt time.Time
}

type ScenarioTestRunWithSummary struct {
	ScenarioTestRun
	Summary []ScenarioTestRunSummary
}

type ScenarioTestRunInput struct {
	ScenarioId         string
	PhantomIterationId string
	EndDate            time.Time
}

func (i ScenarioTestRunInput) CreateDbInput(liveIterationId string) ScenarioTestRunCreateDbInput {
	return ScenarioTestRunCreateDbInput{
		ScenarioId:         i.ScenarioId,
		PhantomIterationId: i.PhantomIterationId,
		LiveScenarioId:     liveIterationId,
		EndDate:            i.EndDate,
	}
}

type ScenarioTestRunCreateDbInput struct {
	ScenarioId         string
	PhantomIterationId string
	LiveScenarioId     string
	EndDate            time.Time
}

type OnCreateIndexesSuccess func(ctx context.Context) error
