package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type ScenarioPublication struct {
	Id                  string    `json:"id"`
	Rank                int32     `json:"rank"`
	ScenarioId          string    `json:"scenario_id"`
	ScenarioIterationId string    `json:"scenario_iteration_id"`
	PublicationAction   string    `json:"publication_action"`
	CreatedAt           time.Time `json:"created_at"`
}

func AdaptScenarioPublicationDto(sp models.ScenarioPublication) ScenarioPublication {
	return ScenarioPublication{
		Id:                  sp.Id,
		Rank:                sp.Rank,
		ScenarioId:          sp.ScenarioId,
		ScenarioIterationId: sp.ScenarioIterationId,
		PublicationAction:   sp.PublicationAction.String(),
		CreatedAt:           sp.CreatedAt,
	}
}

type CreateScenarioPublicationBody struct {
	ScenarioIterationId string `json:"scenario_iteration_id"`
	PublicationAction   string `json:"publication_action"`
	TestMode            bool   `json:"test_mode"`
}

func AdaptCreateScenarioPublicationBody(dto CreateScenarioPublicationBody) models.PublishScenarioIterationInput {
	out := models.PublishScenarioIterationInput{
		ScenarioIterationId: dto.ScenarioIterationId,
		PublicationAction:   models.PublicationActionFrom(dto.PublicationAction),
		TestMode:            dto.TestMode,
	}

	return out
}

type PublicationPreparationStatus struct {
	PreparationStatus        string `json:"preparation_status"`
	PreparationServiceStatus string `json:"preparation_service_status"`
}

func AdaptPublicationPreparationStatus(status models.PublicationPreparationStatus) PublicationPreparationStatus {
	return PublicationPreparationStatus{
		PreparationStatus:        string(status.PreparationStatus),
		PreparationServiceStatus: string(status.PreparationServiceStatus),
	}
}
