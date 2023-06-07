package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type ScenarioIterationUsecase struct {
	scenarioIterationsReadRepository  repositories.ScenarioIterationReadRepository
	scenarioIterationsWriteRepository repositories.ScenarioIterationWriteRepository
}

func (usecase *ScenarioIterationUsecase) ListScenarioIterations(ctx context.Context, organizationID string, filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error) {
	return usecase.scenarioIterationsReadRepository.ListScenarioIterations(ctx, organizationID, filters)
}

func (usecase *ScenarioIterationUsecase) GetScenarioIteration(ctx context.Context, organizationID string, scenarioIterationID string) (models.ScenarioIteration, error) {
	return usecase.scenarioIterationsReadRepository.GetScenarioIteration(ctx, organizationID, scenarioIterationID)
}

func (usecase *ScenarioIterationUsecase) CreateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error) {
	return usecase.scenarioIterationsWriteRepository.CreateScenarioIteration(ctx, organizationID, scenarioIteration)
}

func (usecase *ScenarioIterationUsecase) UpdateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error) {
	return usecase.scenarioIterationsWriteRepository.UpdateScenarioIteration(ctx, organizationID, scenarioIteration)
}
