package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type ScenarioPublication struct {
	Id                         string    `json:"id"`
	Rank                       int32     `json:"rank"`
	ScenarioId_deprec          string    `json:"scenarioID"` //nolint:tagliatelle
	ScenarioId                 string    `json:"scenario_id"`
	ScenarioIterationId_deprec string    `json:"scenarioIterationID"` //nolint:tagliatelle
	ScenarioIterationId        string    `json:"scenario_iteration_id"`
	PublicationAction_deprec   string    `json:"publicationAction"` //nolint:tagliatelle
	PublicationAction          string    `json:"publication_action"`
	CreatedAt_deprec           time.Time `json:"createdAt"` //nolint:tagliatelle
	CreatedAt                  time.Time `json:"created_at"`
}

func AdaptScenarioPublicationDto(sp models.ScenarioPublication) ScenarioPublication {
	return ScenarioPublication{
		Id:                         sp.Id,
		Rank:                       sp.Rank,
		ScenarioId_deprec:          sp.ScenarioId,
		ScenarioId:                 sp.ScenarioId,
		ScenarioIterationId_deprec: sp.ScenarioIterationId,
		ScenarioIterationId:        sp.ScenarioIterationId,
		PublicationAction_deprec:   sp.PublicationAction.String(),
		PublicationAction:          sp.PublicationAction.String(),
		CreatedAt_deprec:           sp.CreatedAt,
		CreatedAt:                  sp.CreatedAt,
	}
}

type CreateScenarioPublicationBody struct {
	ScenarioIterationId_deprec string `json:"scenarioIterationID"` //nolint:tagliatelle
	ScenarioIterationId        string `json:"scenario_iteration_id"`
	PublicationAction_deprec   string `json:"publicationAction"` //nolint:tagliatelle
	PublicationAction          string `json:"publication_action"`
}

func AdaptCreateScenarioPublicationBody(dto CreateScenarioPublicationBody) models.PublishScenarioIterationInput {
	out := models.PublishScenarioIterationInput{
		ScenarioIterationId: dto.ScenarioIterationId,
		PublicationAction:   models.PublicationActionFrom(dto.PublicationAction),
	}

	// TODO remove deprecated fields
	if out.ScenarioIterationId == "" {
		out.ScenarioIterationId = dto.ScenarioIterationId_deprec
	}
	if out.PublicationAction == models.UnknownPublicationAction {
		out.PublicationAction = models.PublicationActionFrom(dto.PublicationAction_deprec)
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
