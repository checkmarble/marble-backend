package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioReadRepository struct {
	mock.Mock
}

func (s *ScenarioReadRepository) GetScenarioById(tx repositories.Transaction, scenarioId string) (models.Scenario, error) {
	args := s.Called(tx, scenarioId)
	return args.Get(0).(models.Scenario), args.Error(1)
}

func (s *ScenarioReadRepository) ListScenariosOfOrganization(tx repositories.Transaction, organizationId string) ([]models.Scenario, error) {
	args := s.Called(tx, organizationId)
	return args.Get(0).([]models.Scenario), args.Error(1)
}

func (s *ScenarioReadRepository) ListAllScenarios(tx repositories.Transaction) ([]models.Scenario, error) {
	args := s.Called(tx)
	return args.Get(0).([]models.Scenario), args.Error(1)
}
