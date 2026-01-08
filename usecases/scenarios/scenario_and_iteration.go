package scenarios

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type ScenarioFetcherRepository interface {
	ListLiveIterationsAndNeighbors(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) ([]models.ScenarioIteration, error)
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string, useCache bool) (
		models.ScenarioIteration, error,
	)
	ListScreeningConfigs(ctx context.Context, exec repositories.Executor,
		scenarioIterationId string, useCache bool) ([]models.ScreeningConfig, error)
	GetScreeningConfig(ctx context.Context, exec repositories.Executor,
		scenarioIterationId, screeningId string) (models.ScreeningConfig, error)
}

type ScenarioFetcher struct {
	Repository ScenarioFetcherRepository
}

func (fetcher ScenarioFetcher) FetchScenarioAndIteration(ctx context.Context,
	exec repositories.Executor, iterationId string,
) (result models.ScenarioAndIteration, err error) {
	result.Iteration, err = fetcher.Repository.GetScenarioIteration(ctx, exec, iterationId, false)
	if err != nil {
		return models.ScenarioAndIteration{}, err
	}

	screeningConfig, err := fetcher.Repository.ListScreeningConfigs(ctx, exec, iterationId, false)
	if err != nil {
		return models.ScenarioAndIteration{}, err
	}
	result.Iteration.ScreeningConfigs = screeningConfig

	result.Scenario, err = fetcher.Repository.GetScenarioById(ctx, exec, result.Iteration.ScenarioId)
	if err != nil {
		return models.ScenarioAndIteration{}, err
	}

	return result, err
}

func (fetcher ScenarioFetcher) ListLiveIterationsAndNeighbors(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID,
) ([]models.ScenarioIteration, error) {
	iterations, err := fetcher.Repository.ListLiveIterationsAndNeighbors(ctx, exec, orgId)
	if err != nil {
		return nil, err
	}

	return iterations, err
}

func (fetcher ScenarioFetcher) FetchScenario(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error) {
	return fetcher.Repository.GetScenarioById(ctx, exec, scenarioId)
}
