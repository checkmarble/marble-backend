package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioRepository struct {
	mock.Mock
}

func (s *ScenarioRepository) GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error) {
	args := s.Called(exec, scenarioId)
	return args.Get(0).(models.Scenario), args.Error(1)
}

func (s *ScenarioRepository) ListScenariosOfOrganization(ctx context.Context, exec repositories.Executor, organizationId string) ([]models.Scenario, error) {
	args := s.Called(exec, organizationId)
	return args.Get(0).([]models.Scenario), args.Error(1)
}

func (s *ScenarioRepository) ListAllScenarios(ctx context.Context, exec repositories.Executor) ([]models.Scenario, error) {
	args := s.Called(exec)
	return args.Get(0).([]models.Scenario), args.Error(1)
}

func (s *ScenarioRepository) CreateScenario(ctx context.Context, exec repositories.Executor, organizationId string, scenario models.CreateScenarioInput, newScenarioId string) error {
	args := s.Called(exec, organizationId, scenario, newScenarioId)
	return args.Error(0)
}

func (s *ScenarioRepository) UpdateScenario(ctx context.Context, exec repositories.Executor, scenario models.UpdateScenarioInput) error {
	args := s.Called(exec, scenario)
	return args.Error(0)
}
