package scenarios

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioFetcherRepository interface {
	GetScenarioById(tx repositories.Transaction, scenarioId string) (models.Scenario, error)
	GetScenarioIteration(tx repositories.Transaction, scenarioIterationId string) (
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

func (fetcher *ScenarioFetcher) FetchScenarioAndIteration(tx repositories.Transaction, iterationId string) (result ScenarioAndIteration, err error) {
	result.Iteration, err = fetcher.Repository.GetScenarioIteration(tx, iterationId)
	if err != nil {
		return ScenarioAndIteration{}, err
	}

	result.Scenario, err = fetcher.Repository.GetScenarioById(tx, result.Iteration.ScenarioId)
	if err != nil {
		return ScenarioAndIteration{}, err
	}

	return result, err
}
