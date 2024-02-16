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
	exec repositories.Executor,
	scenarioAndIteration models.ScenarioAndIteration,
	publicationAction models.PublicationAction,
) ([]models.ScenarioPublication, error) {
	args := m.Called(ctx, exec, scenarioAndIteration, publicationAction)
	return args.Get(0).([]models.ScenarioPublication), args.Error(1)
}
