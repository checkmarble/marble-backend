package scenarios

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioFetcherRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (
		models.ScenarioIteration, error,
	)
}

type ScenarioAndIteration struct {
	Scenario  models.Scenario
	Iteration models.ScenarioIteration
}

type ScenarioFetcher struct {
	Repository ScenarioFetcherRepository
}

func (fetcher *ScenarioFetcher) FetchScenarioAndIteration(ctx context.Context,
	exec repositories.Executor, iterationId string,
) (result ScenarioAndIteration, err error) {
	result.Iteration, err = fetcher.Repository.GetScenarioIteration(ctx, exec, iterationId)
	if err != nil {
		return ScenarioAndIteration{}, err
	}

	result.Scenario, err = fetcher.Repository.GetScenarioById(ctx, exec, result.Iteration.ScenarioId)
	if err != nil {
		return ScenarioAndIteration{}, err
	}

	return result, err
}
