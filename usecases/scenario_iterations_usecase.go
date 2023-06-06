package usecases

import (
	"context"
	"marble/marble-backend/app"
	"marble/marble-backend/repositories"
)

type ScenarioIterationUsecase struct {
	scenarioIterationsReadRepository  repositories.ScenarioIterationReadRepository
	scenarioIterationsWriteRepository repositories.ScenarioIterationWriteRepository
}

func (usecase *ScenarioIterationUsecase) ListScenarioIterations(ctx context.Context, organizationID string, filters app.GetScenarioIterationFilters) ([]app.ScenarioIteration, error) {
	return usecase.scenarioIterationsReadRepository.ListScenarioIterations(ctx, organizationID, filters)
}

func (usecase *ScenarioIterationUsecase) GetScenarioIteration(ctx context.Context, organizationID string, scenarioIterationID string) (app.ScenarioIteration, error) {
	return usecase.scenarioIterationsReadRepository.GetScenarioIteration(ctx, organizationID, scenarioIterationID)
}

func (usecase *ScenarioIterationUsecase) CreateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration app.CreateScenarioIterationInput) (app.ScenarioIteration, error) {
	return usecase.scenarioIterationsWriteRepository.CreateScenarioIteration(ctx, organizationID, scenarioIteration)
}

func (usecase *ScenarioIterationUsecase) UpdateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration app.UpdateScenarioIterationInput) (app.ScenarioIteration, error) {
	return usecase.scenarioIterationsWriteRepository.UpdateScenarioIteration(ctx, organizationID, scenarioIteration)
}
