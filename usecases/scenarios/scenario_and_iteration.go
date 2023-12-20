package scenarios

import (
	"fmt"
	"time"

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

	a := time.Now()
	result.Iteration, err = fetcher.Repository.GetScenarioIteration(tx, iterationId)
	fmt.Println("get scenario iteration", time.Since(a))
	if err != nil {
		return ScenarioAndIteration{}, err
	}

	a = time.Now()
	result.Scenario, err = fetcher.Repository.GetScenarioById(tx, result.Iteration.ScenarioId)
	fmt.Println("get scenario", time.Since(a))
	if err != nil {
		return ScenarioAndIteration{}, err
	}

	return result, err
}
