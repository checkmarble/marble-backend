package usecases

import (
	"context"
	"marble/marble-backend/app"
	"marble/marble-backend/repositories"
)

type ScenarioPublicationUsecase struct {
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
}

func (usecase *ScenarioPublicationUsecase) ListScenarioPublications(ctx context.Context, orgID string, filters app.ListScenarioPublicationsFilters) ([]app.ScenarioPublication, error) {
	return usecase.scenarioPublicationsRepository.ListScenarioPublications(ctx, orgID, filters)
}

func (usecase *ScenarioPublicationUsecase) CreateScenarioPublication(ctx context.Context, orgID string, sp app.CreateScenarioPublicationInput) ([]app.ScenarioPublication, error) {
	return usecase.scenarioPublicationsRepository.CreateScenarioPublication(ctx, orgID, sp)
}

func (usecase *ScenarioPublicationUsecase) GetScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (app.ScenarioPublication, error) {
	return usecase.scenarioPublicationsRepository.GetScenarioPublication(ctx, orgID, scenarioPublicationID)
}
