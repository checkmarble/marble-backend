package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioPublicationRepository struct {
	mock.Mock
}

func (s *ScenarioPublicationRepository) ListScenarioPublicationsOfOrganization(ctx context.Context, exec repositories.Executor, organizationId string, filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error) {
	args := s.Called(exec, organizationId, filters)
	return args.Get(0).([]models.ScenarioPublication), args.Error(1)
}

func (s *ScenarioPublicationRepository) CreateScenarioPublication(ctx context.Context, exec repositories.Executor, input models.CreateScenarioPublicationInput, newScenarioPublicationId string) error {
	args := s.Called(exec, input, newScenarioPublicationId)
	return args.Error(0)
}

func (s *ScenarioPublicationRepository) GetScenarioPublicationById(ctx context.Context, exec repositories.Executor, scenarioPublicationID string) (models.ScenarioPublication, error) {
	args := s.Called(exec, scenarioPublicationID)
	return args.Get(0).(models.ScenarioPublication), args.Error(1)
}
