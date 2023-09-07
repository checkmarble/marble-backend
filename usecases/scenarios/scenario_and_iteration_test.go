package scenarios

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

func TestScenarioFetcher_FetchScenarioAndIteration(t *testing.T) {
	scenario := models.Scenario{
		Id: "scenario_id",
	}

	scenarioIteration := models.ScenarioIteration{
		Id:         "scenario_iteration_id",
		ScenarioId: "scenario_id",
	}

	mt := new(mocks.Transaction)

	mscirr := new(mocks.ScenarioIterationReadRepository)
	mscirr.On("GetScenarioIteration", mt, scenarioIteration.Id).Return(scenarioIteration, nil)

	mrr := new(mocks.ScenarioReadRepository)
	mrr.On("GetScenarioById", mt, scenario.Id).Return(scenario, nil)

	fetcher := ScenarioFetcher{
		ScenarioReadRepository:          mrr,
		ScenarioIterationReadRepository: mscirr,
	}

	result, err := fetcher.FetchScenarioAndIteration(mt, scenarioIteration.Id)
	assert.NoError(t, err)
	assert.Equal(t, ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: scenarioIteration,
	}, result)

	mt.AssertExpectations(t)
	mrr.AssertExpectations(t)
	mscirr.AssertExpectations(t)
}

func TestScenarioFetcher_FetchScenarioAndIteration_GetScenarioIteration_error(t *testing.T) {
	mt := new(mocks.Transaction)

	mscirr := new(mocks.ScenarioIterationReadRepository)
	mscirr.On("GetScenarioIteration", mt, "scenario_iteration_id").Return(models.ScenarioIteration{}, assert.AnError)

	fetcher := ScenarioFetcher{
		ScenarioIterationReadRepository: mscirr,
	}

	_, err := fetcher.FetchScenarioAndIteration(mt, "scenario_iteration_id")
	assert.Error(t, err)

	mt.AssertExpectations(t)
	mscirr.AssertExpectations(t)
}

func TestScenarioFetcher_FetchScenarioAndIteration_GetScenarioById_error(t *testing.T) {
	scenario := models.Scenario{
		Id: "scenario_id",
	}

	scenarioIteration := models.ScenarioIteration{
		Id:         "scenario_iteration_id",
		ScenarioId: "scenario_id",
	}

	mt := new(mocks.Transaction)

	mscirr := new(mocks.ScenarioIterationReadRepository)
	mscirr.On("GetScenarioIteration", mt, scenarioIteration.Id).Return(scenarioIteration, nil)

	mrr := new(mocks.ScenarioReadRepository)
	mrr.On("GetScenarioById", mt, scenario.Id).Return(scenario, assert.AnError)

	fetcher := ScenarioFetcher{
		ScenarioReadRepository:          mrr,
		ScenarioIterationReadRepository: mscirr,
	}

	_, err := fetcher.FetchScenarioAndIteration(mt, scenarioIteration.Id)
	assert.Error(t, err)

	mt.AssertExpectations(t)
	mrr.AssertExpectations(t)
	mscirr.AssertExpectations(t)
}
