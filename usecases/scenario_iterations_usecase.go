package usecases

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"

	"github.com/adhocore/gronx"
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
	body := scenarioIteration.Body
	if body != nil && body.Schedule != "" {
		gron := gronx.New()
		ok := gron.IsValid(body.Schedule)
		if !ok {
			return models.ScenarioIteration{}, fmt.Errorf("Invalid schedule: %w", models.BadParameterError)
		}
	}
	return usecase.scenarioIterationsWriteRepository.CreateScenarioIteration(ctx, organizationID, scenarioIteration)
}

func (usecase *ScenarioIterationUsecase) UpdateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error) {
	body := scenarioIteration.Body
	if body != nil && body.Schedule != nil && *body.Schedule != "" {
		gron := gronx.New()
		ok := gron.IsValid(*body.Schedule)
		if !ok {
			return models.ScenarioIteration{}, fmt.Errorf("Invalid schedule: %w", models.BadParameterError)
		}
	}
	return usecase.scenarioIterationsWriteRepository.UpdateScenarioIteration(ctx, organizationID, scenarioIteration)
}
