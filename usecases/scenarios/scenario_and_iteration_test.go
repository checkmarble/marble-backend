package scenarios

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioFetcherRepositoryMock struct {
	mock.Mock
}

func (s *ScenarioFetcherRepositoryMock) GetScenarioById(ctx context.Context, tx repositories.Transaction_deprec, scenarioId string) (models.Scenario, error) {
	args := s.Called(tx, scenarioId)
	return args.Get(0).(models.Scenario), args.Error(1)
}

func (s *ScenarioFetcherRepositoryMock) GetScenarioIteration(ctx context.Context, tx repositories.Transaction_deprec, scenarioIterationId string) (models.ScenarioIteration, error) {
	args := s.Called(tx, scenarioIterationId)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}

func TestScenarioFetcher_FetchScenarioAndIteration(t *testing.T) {
	scenario := models.Scenario{
		Id: "scenario_id",
	}

	scenarioIteration := models.ScenarioIteration{
		Id:         "scenario_iteration_id",
		ScenarioId: "scenario_id",
	}

	mt := new(mocks.Transaction)

	repo := new(ScenarioFetcherRepositoryMock)
	repo.On("GetScenarioIteration", mt, scenarioIteration.Id).Return(scenarioIteration, nil)
	repo.On("GetScenarioById", mt, scenario.Id).Return(scenario, nil)

	fetcher := ScenarioFetcher{
		Repository: repo,
	}

	result, err := fetcher.FetchScenarioAndIteration(context.TODO(), mt, scenarioIteration.Id)
	assert.NoError(t, err)
	assert.Equal(t, ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: scenarioIteration,
	}, result)

	mt.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestScenarioFetcher_FetchScenarioAndIteration_GetScenarioIteration_error(t *testing.T) {
	mt := new(mocks.Transaction)

	repo := new(ScenarioFetcherRepositoryMock)
	repo.On("GetScenarioIteration", mt, "scenario_iteration_id").Return(models.ScenarioIteration{}, assert.AnError)

	fetcher := ScenarioFetcher{
		Repository: repo,
	}

	_, err := fetcher.FetchScenarioAndIteration(context.TODO(), mt, "scenario_iteration_id")
	assert.Error(t, err)

	mt.AssertExpectations(t)
	repo.AssertExpectations(t)
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

	repo := new(ScenarioFetcherRepositoryMock)
	repo.On("GetScenarioIteration", mt, scenarioIteration.Id).Return(scenarioIteration, nil)
	repo.On("GetScenarioById", mt, scenario.Id).Return(scenario, assert.AnError)

	fetcher := ScenarioFetcher{
		Repository: repo,
	}

	_, err := fetcher.FetchScenarioAndIteration(context.TODO(), mt, scenarioIteration.Id)
	assert.Error(t, err)

	mt.AssertExpectations(t)
	repo.AssertExpectations(t)
}
