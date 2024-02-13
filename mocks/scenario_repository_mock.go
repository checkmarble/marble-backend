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

func (s *ScenarioRepository) GetScenarioById(ctx context.Context, tx repositories.Transaction_deprec, scenarioId string) (models.Scenario, error) {
	args := s.Called(tx, scenarioId)
	return args.Get(0).(models.Scenario), args.Error(1)
}

func (s *ScenarioRepository) ListScenariosOfOrganization(ctx context.Context, tx repositories.Transaction_deprec, organizationId string) ([]models.Scenario, error) {
	args := s.Called(tx, organizationId)
	return args.Get(0).([]models.Scenario), args.Error(1)
}

func (s *ScenarioRepository) ListAllScenarios(ctx context.Context, tx repositories.Transaction_deprec) ([]models.Scenario, error) {
	args := s.Called(tx)
	return args.Get(0).([]models.Scenario), args.Error(1)
}

func (s *ScenarioRepository) CreateScenario(ctx context.Context, tx repositories.Transaction_deprec, organizationId string, scenario models.CreateScenarioInput, newScenarioId string) error {
	args := s.Called(tx, organizationId, scenario, newScenarioId)
	return args.Error(0)
}

func (s *ScenarioRepository) UpdateScenario(ctx context.Context, tx repositories.Transaction_deprec, scenario models.UpdateScenarioInput) error {
	args := s.Called(tx, scenario)
	return args.Error(0)
}
