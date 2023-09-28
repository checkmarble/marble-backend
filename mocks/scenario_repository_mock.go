package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioRepository struct {
	mock.Mock
}

func (s *ScenarioRepository) GetScenarioById(tx repositories.Transaction, scenarioId string) (models.Scenario, error) {
	args := s.Called(tx, scenarioId)
	return args.Get(0).(models.Scenario), args.Error(1)
}

func (s *ScenarioRepository) ListScenariosOfOrganization(tx repositories.Transaction, organizationId string) ([]models.Scenario, error) {
	args := s.Called(tx, organizationId)
	return args.Get(0).([]models.Scenario), args.Error(1)
}

func (s *ScenarioRepository) ListAllScenarios(tx repositories.Transaction) ([]models.Scenario, error) {
	args := s.Called(tx)
	return args.Get(0).([]models.Scenario), args.Error(1)
}

func (s *ScenarioRepository) CreateScenario(tx repositories.Transaction, organizationId string, scenario models.CreateScenarioInput, newScenarioId string) error {
	args := s.Called(tx, organizationId, scenario, newScenarioId)
	return args.Error(0)
}

func (s *ScenarioRepository) UpdateScenario(tx repositories.Transaction, scenario models.UpdateScenarioInput) error {
	args := s.Called(tx, scenario)
	return args.Error(0)
}
