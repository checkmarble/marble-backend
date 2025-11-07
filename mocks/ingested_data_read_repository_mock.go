package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScreeningMonitoringIngestedDataReader struct {
	mock.Mock
}

func (m *ScreeningMonitoringIngestedDataReader) QueryIngestedObject(
	ctx context.Context,
	exec repositories.Executor,
	table models.Table,
	objectId string,
) ([]models.DataModelObject, error) {
	args := m.Called(ctx, exec, table, objectId)
	return args.Get(0).([]models.DataModelObject), args.Error(1)
}
