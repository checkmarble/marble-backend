package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
)

type ScheduledExecution struct {
	Id               string           `json:"id"`
	Scenario         DecisionScenario `json:"scenario"`
	Manual           bool             `json:"manual"`
	Status           string           `json:"status"`
	DecisionsCreated int              `json:"decisions_created"`
	CreatedAt        pubapi.DateTime  `json:"created_at"`
	FinishedAt       *pubapi.DateTime `json:"finished_at"`
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
		CreatedAt:        pubapi.DateTime(model.StartedAt),
		FinishedAt:       pubapi.ThenDateTime(model.FinishedAt),
	}
}
