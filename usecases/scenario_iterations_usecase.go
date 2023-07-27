package usecases

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/security"

	"github.com/adhocore/gronx"
)

type ScenarioIterationUsecase struct {
	organizationIdOfContext           func() (string, error)
	scenarioIterationsReadRepository  repositories.ScenarioIterationReadRepository
	scenarioIterationsWriteRepository repositories.ScenarioIterationWriteRepository
	enforceSecurity                   security.EnforceSecurityScenario
}

func (usecase *ScenarioIterationUsecase) ListScenarioIterations(filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error) {
	organizationId, err := usecase.organizationIdOfContext()
	scenarioIterations, err := usecase.scenarioIterationsReadRepository.ListScenarioIterations(nil, organizationId, filters)
	if err != nil {
		return nil, err
	}
	for _, si := range scenarioIterations {
		if err := usecase.enforceSecurity.ReadScenarioIteration(si); err != nil {
			return nil, err
		}
	}
	return scenarioIterations, nil
}

func (usecase *ScenarioIterationUsecase) GetScenarioIteration(scenarioIterationID string) (models.ScenarioIteration, error) {
	si, err := usecase.scenarioIterationsReadRepository.GetScenarioIteration(nil, scenarioIterationID)
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	if err := usecase.enforceSecurity.ReadScenarioIteration(si); err != nil {
		return models.ScenarioIteration{}, err
	}
	return si, nil
}

func (usecase *ScenarioIterationUsecase) CreateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error) {
	body := scenarioIteration.Body
	if body != nil && body.Schedule != "" {
		gron := gronx.New()
		ok := gron.IsValid(body.Schedule)
		if !ok {
			return models.ScenarioIteration{}, fmt.Errorf("invalid schedule: %w", models.BadParameterError)
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
			return models.ScenarioIteration{}, fmt.Errorf("invalid schedule: %w", models.BadParameterError)
		}
	}
	return usecase.scenarioIterationsWriteRepository.UpdateScenarioIteration(ctx, organizationID, scenarioIteration)
}
