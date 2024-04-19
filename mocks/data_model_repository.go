package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
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
) (models.DataModel, error) {
	args := d.Called(ctx, exec, organizationId, fetchEnumValues)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (d *DataModelRepository) DeleteDataModel(ctx context.Context, exec repositories.Executor, organizationId string) error {
	args := d.Called(ctx, exec, organizationId)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModelTable(ctx context.Context, exec repositories.Executor, organizationID, tableID, name, description string) error {
	args := d.Called(ctx, exec, organizationID, tableID, name, description)
	return args.Error(0)
}

func (d *DataModelRepository) UpdateDataModelTable(ctx context.Context, exec repositories.Executor, tableID, description string) error {
	args := d.Called(ctx, exec, tableID, description)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModelField(
	ctx context.Context,
	exec repositories.Executor,
	tableID string,
	field models.CreateFieldInput,
) error {
	args := d.Called(ctx, exec, tableID, field)
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

func (d *DataModelRepository) CreateDataModelLink(ctx context.Context, exec repositories.Executor, link models.DataModelLinkCreateInput) error {
	args := d.Called(ctx, exec, link)
	return args.Error(0)
}

func (d *DataModelRepository) UpdateDataModelField(ctx context.Context, exec repositories.Executor,
	fieldID string, input models.UpdateFieldInput,
) error {
	args := d.Called(ctx, exec, fieldID, input)
	return args.Error(0)
}

func (repo *DataModelRepository) GetDataModelField(ctx context.Context, exec repositories.Executor, fieldId string) (models.FieldMetadata, error) {
	args := repo.Called(ctx, exec, fieldId)
	return args.Get(0).(models.FieldMetadata), args.Error(1)
}

func (d *DataModelRepository) CreatePivot(ctx context.Context, exec repositories.Executor, id string, pivot models.CreatePivotInput) error {
	args := d.Called(ctx, exec, id, pivot)
	return args.Error(0)
}

func (d *DataModelRepository) ListPivots(ctx context.Context, exec repositories.Executor,
	organization_id string, tableId *string,
) ([]models.PivotMetadata, error) {
	args := d.Called(ctx, exec, organization_id, tableId)
	return args.Get(0).([]models.PivotMetadata), args.Error(1)
}

func (d *DataModelRepository) GetPivot(ctx context.Context, exec repositories.Executor, pivotId string) (models.PivotMetadata, error) {
	args := d.Called(ctx, exec, pivotId)
	return args.Get(0).(models.PivotMetadata), args.Error(1)
}
