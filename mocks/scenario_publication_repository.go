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

func (s *ScenarioPublicationRepository) ListScenarioPublicationsOfOrganization(ctx context.Context, tx repositories.Transaction, organizationId string, filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error) {
	args := s.Called(tx, organizationId, filters)
	return args.Get(0).([]models.ScenarioPublication), args.Error(1)
}

func (s *ScenarioPublicationRepository) CreateScenarioPublication(ctx context.Context, tx repositories.Transaction, input models.CreateScenarioPublicationInput, newScenarioPublicationId string) error {
	args := s.Called(tx, input, newScenarioPublicationId)
	return args.Error(0)
}

func (s *ScenarioPublicationRepository) GetScenarioPublicationById(ctx context.Context, tx repositories.Transaction, scenarioPublicationID string) (models.ScenarioPublication, error) {
	args := s.Called(tx, scenarioPublicationID)
	return args.Get(0).(models.ScenarioPublication), args.Error(1)
}
