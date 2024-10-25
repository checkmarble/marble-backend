package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioPublisher struct {
	mock.Mock
}

func (m *ScenarioPublisher) PublishOrUnpublishIteration(
	ctx context.Context,
	tx repositories.Transaction,
	scenarioAndIteration models.ScenarioAndIteration,
	publicationAction models.PublicationAction,
	testMode bool,
) ([]models.ScenarioPublication, error) {
	args := m.Called(ctx, tx, scenarioAndIteration, publicationAction)
	return args.Get(0).([]models.ScenarioPublication), args.Error(1)
}
