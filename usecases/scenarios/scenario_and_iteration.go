package scenarios

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type ScenarioAndIteration struct {
	Scenario  models.Scenario
	Iteration models.ScenarioIteration
}

type ScenarioFetcher struct {
	ScenarioReadRepository          repositories.ScenarioReadRepository
	ScenarioIterationReadRepository repositories.ScenarioIterationReadRepository
}

func (fetcher *ScenarioFetcher) FetchScenarioAndIteration(tx repositories.Transaction, iterationId string) (result ScenarioAndIteration, err error) {

	result.Iteration, err = fetcher.ScenarioIterationReadRepository.GetScenarioIteration(tx, iterationId)
	if err != nil {
		return ScenarioAndIteration{}, err
	}

	result.Scenario, err = fetcher.ScenarioReadRepository.GetScenarioById(tx, result.Iteration.ScenarioId)
	if err != nil {
		return ScenarioAndIteration{}, err
	}

	return result, err
}