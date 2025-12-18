package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
)

type DataModelRepository struct {
	mock.Mock
}

func (d *DataModelRepository) GetDataModel(
	ctx context.Context,
	exec repositories.Executor,
	organizationId string,
	fetchEnumValues bool,
	useCache bool,
) (models.DataModel, error) {
	args := d.Called(ctx, exec, organizationId, fetchEnumValues, useCache)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (d *DataModelRepository) DeleteDataModel(ctx context.Context, exec repositories.Executor, organizationId string) error {
	args := d.Called(ctx, exec, organizationId)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModelTable(
	ctx context.Context,
	exec repositories.Executor,
	organizationID, tableID, name, description string,
	ftmEntity *models.FollowTheMoneyEntity,
) error {
	args := d.Called(ctx, exec, organizationID, tableID, name, description, ftmEntity)
	return args.Error(0)
}

func (d *DataModelRepository) UpdateDataModelTable(
	ctx context.Context,
	exec repositories.Executor,
	tableID string,
	description *string,
	ftmEntity pure_utils.Null[models.FollowTheMoneyEntity],
) error {
	args := d.Called(ctx, exec, tableID, description, ftmEntity)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModelField(
	ctx context.Context,
	exec repositories.Executor,
	organizationId string,
	tableID string,
	field models.CreateFieldInput,
) error {
	args := d.Called(ctx, exec, organizationId, tableID, field)
	return args.Error(0)
}

func (d *DataModelRepository) GetLinks(ctx context.Context, exec repositories.Executor, organizationID string) ([]models.LinkToSingle, error) {
	args := d.Called(ctx, exec, organizationID)
	return args.Get(0).([]models.LinkToSingle), args.Error(1)
}

func (d *DataModelRepository) GetDataModelTable(ctx context.Context, exec repositories.Executor, tableID string) (models.TableMetadata, error) {
	args := d.Called(ctx, exec, tableID)
	return args.Get(0).(models.TableMetadata), args.Error(1)
}

func (d *DataModelRepository) CreateDataModelLink(ctx context.Context, exec repositories.Executor, id string, link models.DataModelLinkCreateInput) error {
	args := d.Called(ctx, exec, id, link)
	return args.Error(0)
}

func (d *DataModelRepository) UpdateDataModelField(ctx context.Context, exec repositories.Executor,
	fieldID string, input models.UpdateFieldInput,
) error {
	args := d.Called(ctx, exec, fieldID, input)
	return args.Error(0)
}

func (d *DataModelRepository) GetDataModelField(ctx context.Context, exec repositories.Executor, fieldId string) (models.FieldMetadata, error) {
	args := d.Called(ctx, exec, fieldId)
	return args.Get(0).(models.FieldMetadata), args.Error(1)
}

func (d *DataModelRepository) CreatePivot(ctx context.Context, exec repositories.Executor, id string, pivot models.CreatePivotInput) error {
	args := d.Called(ctx, exec, id, pivot)
	return args.Error(0)
}

func (d *DataModelRepository) ListPivots(
	ctx context.Context,
	exec repositories.Executor,
	organization_id string,
	tableId *string,
	useCache bool,
) ([]models.PivotMetadata, error) {
	args := d.Called(ctx, exec, organization_id, tableId, useCache)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	if args.Get(0) == nil {
		return []models.PivotMetadata{}, nil
	}
	return args.Get(0).([]models.PivotMetadata), args.Error(1)
}

func (d *DataModelRepository) GetPivot(ctx context.Context, exec repositories.Executor, pivotId string) (models.PivotMetadata, error) {
	args := d.Called(ctx, exec, pivotId)
	return args.Get(0).(models.PivotMetadata), args.Error(1)
}

func (d *DataModelRepository) BatchInsertEnumValues(ctx context.Context, exec repositories.Executor, enumValues models.EnumValues, table models.Table) error {
	args := d.Called(ctx, exec, enumValues, table)
	return args.Error(0)
}

func (d *DataModelRepository) GetDataModelOptionsForTable(ctx context.Context,
	exec repositories.Executor, tableId string,
) (*models.DataModelOptions, error) {
	args := d.Called(ctx, exec, tableId)
	return args.Get(0).(*models.DataModelOptions), args.Error(1)
}

func (d *DataModelRepository) UpsertDataModelOptions(ctx context.Context, exec repositories.Executor,
	req models.UpdateDataModelOptionsRequest,
) (models.DataModelOptions, error) {
	args := d.Called(ctx, exec, req)
	return args.Get(0).(models.DataModelOptions), args.Error(1)
}

func (d *DataModelRepository) ArchiveDataModelField(ctx context.Context, exec repositories.Executor, table models.TableMetadata, field models.FieldMetadata) error {
	args := d.Called(ctx, exec, table, field)
	return args.Error(1)
}

func (d *DataModelRepository) DeleteDataModelField(ctx context.Context, exec repositories.Executor, table models.TableMetadata, field models.FieldMetadata) error {
	args := d.Called(ctx, exec, table, field)
	return args.Error(0)
}
