package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type ScheduledExecution struct {
	Id               string           `json:"id"`
	Scenario         DecisionScenario `json:"scenario"`
	Manual           bool             `json:"manual"`
	Status           string           `json:"status"`
	DecisionsCreated int              `json:"decisions_created"`
	CreatedAt        time.Time        `json:"created_at"`
	FinishedAt       *time.Time       `json:"finished_at"`
}

func AdaptScheduledExecution(model models.ScheduledExecution) ScheduledExecution {
	return ScheduledExecution{
		Id: model.Id,
		Scenario: DecisionScenario{
			Id:          model.ScenarioId,
			IterationId: model.ScenarioIterationId,
			Version:     model.ScenarioVersion,
		},
		Manual:           model.Manual,
		Status:           model.Status.String(),
		DecisionsCreated: model.NumberOfCreatedDecisions,
		CreatedAt:        model.StartedAt,
		FinishedAt:       model.FinishedAt,
	}
}
