package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ContinuousScreeningIngestedDataReader struct {
	mock.Mock
}

func (m *ContinuousScreeningIngestedDataReader) QueryIngestedObject(
	ctx context.Context,
	exec repositories.Executor,
	table models.Table,
	objectId string,
	metadataFields ...string,
) ([]models.DataModelObject, error) {
	args := m.Called(ctx, exec, table, objectId, metadataFields)
	return args.Get(0).([]models.DataModelObject), args.Error(1)
}
