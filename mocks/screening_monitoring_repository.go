package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type ScreeningMonitoringRepository struct {
	mock.Mock
}

func (m *ScreeningMonitoringRepository) GetScreeningMonitoringConfig(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
) (models.ScreeningMonitoringConfig, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.ScreeningMonitoringConfig), args.Error(1)
}

func (m *ScreeningMonitoringRepository) GetScreeningMonitoringConfigsByOrgId(
	ctx context.Context,
	exec repositories.Executor,
	orgId string,
) ([]models.ScreeningMonitoringConfig, error) {
	args := m.Called(ctx, exec, orgId)
	return args.Get(0).([]models.ScreeningMonitoringConfig), args.Error(1)
}

func (m *ScreeningMonitoringRepository) CreateScreeningMonitoringConfig(
	ctx context.Context,
	exec repositories.Executor,
	input models.CreateScreeningMonitoringConfig,
) (models.ScreeningMonitoringConfig, error) {
	args := m.Called(ctx, exec, input)
	return args.Get(0).(models.ScreeningMonitoringConfig), args.Error(1)
}

func (m *ScreeningMonitoringRepository) UpdateScreeningMonitoringConfig(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
	input models.UpdateScreeningMonitoringConfig,
) (models.ScreeningMonitoringConfig, error) {
	args := m.Called(ctx, exec, id, input)
	return args.Get(0).(models.ScreeningMonitoringConfig), args.Error(1)
}

func (m *ScreeningMonitoringRepository) GetDataModelTable(
	ctx context.Context,
	exec repositories.Executor,
	tableId string,
) (models.TableMetadata, error) {
	args := m.Called(ctx, exec, tableId)
	return args.Get(0).(models.TableMetadata), args.Error(1)
}

func (m *ScreeningMonitoringRepository) GetDataModel(
	ctx context.Context,
	exec repositories.Executor,
	organizationID string,
	fetchEnumValues bool,
	useCache bool,
) (models.DataModel, error) {
	args := m.Called(ctx, exec, organizationID, fetchEnumValues, useCache)
	return args.Get(0).(models.DataModel), args.Error(1)
}

type ScreeningMonitoringClientDbRepository struct {
	mock.Mock
}

func (m *ScreeningMonitoringClientDbRepository) CreateInternalScreeningMonitoringTable(
	ctx context.Context,
	exec repositories.Executor,
	tableName string,
) error {
	args := m.Called(ctx, exec, tableName)
	return args.Error(0)
}

func (m *ScreeningMonitoringClientDbRepository) InsertScreeningMonitoringObject(
	ctx context.Context,
	exec repositories.Executor,
	tableName string,
	objectId string,
	configId uuid.UUID,
) error {
	args := m.Called(ctx, exec, tableName, objectId, configId)
	return args.Error(0)
}
