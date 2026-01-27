package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
)

type ScheduledExecution struct {
	Id               string           `json:"id"`
	Scenario         DecisionScenario `json:"scenario"`
	Manual           bool             `json:"manual"`
	Status           string           `json:"status"`
	DecisionsCreated int              `json:"decisions_created"`
	CreatedAt        types.DateTime   `json:"created_at"`
	FinishedAt       *types.DateTime  `json:"finished_at"`
}

func (ScheduledExecution) ApiVersion() string {
	return "v1"
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
		CreatedAt:        types.DateTime(model.StartedAt),
		FinishedAt:       types.ThenDateTime(model.FinishedAt),
	}
}
