package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioListRepository struct {
	mock.Mock
}

func (m *ScenarioListRepository) ListScenariosOfOrganization(ctx context.Context,
	exec repositories.Executor, organizationId string,
) ([]models.Scenario, error) {
	args := m.Called(ctx, exec, organizationId)
	return args.Get(0).([]models.Scenario), args.Error(1)
}
