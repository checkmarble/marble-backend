package dto

import "github.com/checkmarble/marble-backend/models"

type CreateScenarioPublicationBody struct {
	ScenarioIterationId string `json:"scenarioIterationID"`
	PublicationAction   string `json:"publicationAction"`
}

type CreateScenarioPublicationInput struct {
	Body *CreateScenarioPublicationBody `in:"body=json"`
}

type ListScenarioPublicationsInput struct {
	ScenarioId          *string `in:"query=scenarioID"`
	ScenarioIterationId *string `in:"query=scenarioIterationID"`
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
