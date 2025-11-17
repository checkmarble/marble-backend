package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type ContinuousScreeningRepository struct {
	mock.Mock
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
) (models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningConfigsByOrgId(
	ctx context.Context,
	exec repositories.Executor,
	orgId string,
) ([]models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, orgId)
	return args.Get(0).([]models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetContinuousScreeningConfigByStableId(
	ctx context.Context,
	exec repositories.Executor,
	stableId uuid.UUID,
) (models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, stableId)
	return args.Get(0).(models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) CreateContinuousScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	input models.CreateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, input)
	return args.Get(0).(models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) UpdateContinuousScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
	input models.UpdateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	args := m.Called(ctx, exec, id, input)
	return args.Get(0).(models.ContinuousScreeningConfig), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetDataModelTable(
	ctx context.Context,
	exec repositories.Executor,
	tableId string,
) (models.TableMetadata, error) {
	args := m.Called(ctx, exec, tableId)
	return args.Get(0).(models.TableMetadata), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetDataModel(
	ctx context.Context,
	exec repositories.Executor,
	organizationID string,
	fetchEnumValues bool,
	useCache bool,
) (models.DataModel, error) {
	args := m.Called(ctx, exec, organizationID, fetchEnumValues, useCache)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (m *ContinuousScreeningRepository) InsertContinuousScreening(
	ctx context.Context,
	exec repositories.Executor,
	screening models.ScreeningWithMatches,
	orgId uuid.UUID,
	configId uuid.UUID,
	configStableId uuid.UUID,
	objectType string,
	objectId string,
	objectInternalId uuid.UUID,
	triggerType models.ContinuousScreeningTriggerType,
) (models.ContinuousScreeningWithMatches, error) {
	args := m.Called(ctx, exec, screening, orgId, configId, configStableId, objectType,
		objectId, objectInternalId, triggerType)
	return args.Get(0).(models.ContinuousScreeningWithMatches), args.Error(1)
}

func (m *ContinuousScreeningRepository) ListContinuousScreeningsForOrg(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ContinuousScreeningWithMatches, error) {
	args := m.Called(ctx, exec, orgId, paginationAndSorting)
	return args.Get(0).([]models.ContinuousScreeningWithMatches), args.Error(1)
}

func (m *ContinuousScreeningRepository) GetInboxById(
	ctx context.Context,
	exec repositories.Executor,
	inboxId uuid.UUID,
) (models.Inbox, error) {
	args := m.Called(ctx, exec, inboxId)
	return args.Get(0).(models.Inbox), args.Error(1)
}

type ContinuousScreeningClientDbRepository struct {
	mock.Mock
}

func (m *ContinuousScreeningClientDbRepository) CreateInternalContinuousScreeningTable(
	ctx context.Context,
	exec repositories.Executor,
	tableName string,
) error {
	args := m.Called(ctx, exec, tableName)
	return args.Error(0)
}

func (m *ContinuousScreeningClientDbRepository) InsertContinuousScreeningObject(
	ctx context.Context,
	exec repositories.Executor,
	tableName string,
	objectId string,
	configStableId uuid.UUID,
) error {
	args := m.Called(ctx, exec, tableName, objectId, configStableId)
	return args.Error(0)
}

type ContinuousScreeningScreeningProvider struct {
	mock.Mock
}

func (m *ContinuousScreeningScreeningProvider) Search(
	ctx context.Context,
	query models.OpenSanctionsQuery,
) (models.ScreeningRawSearchResponseWithMatches, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return models.ScreeningRawSearchResponseWithMatches{}, args.Error(1)
	}
	return args.Get(0).(models.ScreeningRawSearchResponseWithMatches), args.Error(1)
}

func (m *ContinuousScreeningScreeningProvider) GetAlgorithms(ctx context.Context) (models.OpenSanctionAlgorithms, error) {
	args := m.Called(ctx)
	return args.Get(0).(models.OpenSanctionAlgorithms), args.Error(1)
}
