package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ObjectMetadataRepository struct {
	mock.Mock
}

func (m *ObjectMetadataRepository) GetDataModel(
	ctx context.Context,
	exec repositories.Executor,
	organizationID uuid.UUID,
	fetchEnumValues bool,
	useCache bool,
) (models.DataModel, error) {
	args := m.Called(ctx, exec, organizationID, fetchEnumValues, useCache)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (m *ObjectMetadataRepository) GetObjectMetadata(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	objectType string,
	objectId string,
	metadataType models.MetadataType,
) (models.ObjectMetadata, error) {
	args := m.Called(ctx, exec, orgId, objectType, objectId, metadataType)
	return args.Get(0).(models.ObjectMetadata), args.Error(1)
}

func (m *ObjectMetadataRepository) UpsertObjectMetadata(
	ctx context.Context,
	exec repositories.Executor,
	input models.ObjectMetadataUpsert,
) (models.ObjectMetadata, error) {
	args := m.Called(ctx, exec, input)
	return args.Get(0).(models.ObjectMetadata), args.Error(1)
}
