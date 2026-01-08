package scenarios

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioFetcherRepositoryMock struct {
	mock.Mock
}

func (s *ScenarioFetcherRepositoryMock) GetScenarioById(ctx context.Context,
	exec repositories.Executor, scenarioId string,
) (models.Scenario, error) {
	args := s.Called(exec, scenarioId)
	return args.Get(0).(models.Scenario), args.Error(1)
}

func (s *ScenarioFetcherRepositoryMock) GetScenarioIteration(ctx context.Context,
	exec repositories.Executor, scenarioIterationId string, useCache bool,
) (models.ScenarioIteration, error) {
	args := s.Called(exec, scenarioIterationId, useCache)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioFetcherRepositoryMock) ListScreeningConfigs(ctx context.Context,
	exec repositories.Executor, scenarioIterationId string, useCache bool,
) ([]models.ScreeningConfig, error) {
	args := s.Called(exec, scenarioIterationId, useCache)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ScreeningConfig), args.Error(1)
}

func (s *ScenarioFetcherRepositoryMock) GetScreeningConfig(ctx context.Context,
	exec repositories.Executor, scenarioIterationId, screeningId string,
) (models.ScreeningConfig, error) {
	args := s.Called(exec, scenarioIterationId)
	if args.Get(0) == nil {
		return models.ScreeningConfig{}, args.Error(1)
	}
	return args.Get(0).(models.ScreeningConfig), args.Error(1)
}

func (s *ScenarioFetcherRepositoryMock) ListLiveIterationsAndNeighbors(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID,
) ([]models.ScenarioIteration, error) {
	args := s.Called(ctx, exec, orgId)

	return []models.ScenarioIteration{}, args.Error(1)
}

func TestScenarioFetcher_FetchScenarioAndIteration(t *testing.T) {
	scenario := models.Scenario{
		Id: "scenario_id",
	}

	scenarioIteration := models.ScenarioIteration{
		Id:         "scenario_iteration_id",
		ScenarioId: "scenario_id",
	}

	mt := new(mocks.Executor)

	repo := new(ScenarioFetcherRepositoryMock)
	repo.On("GetScenarioIteration", mt, scenarioIteration.Id, mock.Anything).Return(scenarioIteration, nil)
	repo.On("GetScenarioById", mt, scenario.Id, mock.Anything).Return(scenario, nil)
	repo.On("ListScreeningConfigs", mt, scenarioIteration.Id, mock.Anything, mock.Anything).Return(nil, nil)

	fetcher := ScenarioFetcher{
		Repository: repo,
	}

	result, err := fetcher.FetchScenarioAndIteration(context.TODO(), mt, scenarioIteration.Id)
	assert.NoError(t, err)
	assert.Equal(t, models.ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: scenarioIteration,
	}, result)

	mt.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestScenarioFetcher_FetchScenarioAndIteration_withScreening(t *testing.T) {
	scenario := models.Scenario{
		Id: "scenario_id",
	}

	scenarioIteration := models.ScenarioIteration{
		Id:               "scenario_iteration_id",
		ScenarioId:       "scenario_id",
		ScreeningConfigs: []models.ScreeningConfig{},
	}

	mt := new(mocks.Executor)

	repo := new(ScenarioFetcherRepositoryMock)
	repo.On("GetScenarioIteration", mt, scenarioIteration.Id, mock.Anything).Return(scenarioIteration, nil)
	repo.On("GetScenarioById", mt, scenario.Id).Return(scenario, nil)
	repo.On("ListScreeningConfigs", mt, scenarioIteration.Id, mock.Anything, mock.Anything).Return([]models.ScreeningConfig{}, nil)

	fetcher := ScenarioFetcher{
		Repository: repo,
	}

	result, err := fetcher.FetchScenarioAndIteration(context.TODO(), mt, scenarioIteration.Id)
	assert.NoError(t, err)
	assert.Equal(t, models.ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: scenarioIteration,
	}, result)

	mt.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestScenarioFetcher_FetchScenarioAndIteration_GetScenarioIteration_error(t *testing.T) {
	mt := new(mocks.Executor)

	repo := new(ScenarioFetcherRepositoryMock)
	repo.On("GetScenarioIteration", mt, "scenario_iteration_id", mock.Anything).Return(
		models.ScenarioIteration{}, assert.AnError)

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

	mt := new(mocks.Executor)

	repo := new(ScenarioFetcherRepositoryMock)
	repo.On("GetScenarioIteration", mt, scenarioIteration.Id, mock.Anything).Return(scenarioIteration, nil)
	repo.On("GetScenarioById", mt, scenario.Id).Return(scenario, assert.AnError)
	repo.On("ListScreeningConfigs", mt, scenarioIteration.Id, mock.Anything, mock.Anything).Return(nil, nil)

	fetcher := ScenarioFetcher{
		Repository: repo,
	}

	_, err := fetcher.FetchScenarioAndIteration(context.TODO(), mt, scenarioIteration.Id)
	assert.Error(t, err)

	mt.AssertExpectations(t)
	repo.AssertExpectations(t)
}
