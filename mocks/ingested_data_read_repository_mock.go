package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
)

type IngestedDataReader struct {
	mock.Mock
}

func (m *IngestedDataReader) GetDbField(ctx context.Context, exec repositories.Executor, readParams models.DbFieldReadParams) (any, error) {
	args := m.Called(ctx, exec, readParams)
	return args.Get(0), args.Error(1)
}

func (m *IngestedDataReader) ListAllObjectIdsFromTable(ctx context.Context,
	exec repositories.Executor, tableName string, filters ...models.Filter,
) ([]string, error) {
	args := m.Called(ctx, exec, tableName, filters)
	return args.Get(0).([]string), args.Error(1)
}

func (m *IngestedDataReader) QueryIngestedObjectByInternalId(ctx context.Context,
	exec repositories.Executor, table models.Table, internalObjectId uuid.UUID, metadataFields ...string,
) (models.DataModelObject, error) {
	args := m.Called(ctx, exec, table, internalObjectId, metadataFields)
	return args.Get(0).(models.DataModelObject), args.Error(1)
}

func (m *IngestedDataReader) QueryIngestedObjectByInternalIds(ctx context.Context,
	exec repositories.Executor, table models.Table, internalObjectIds []uuid.UUID,
	metadataFields ...string,
) ([]models.DataModelObject, error) {
	args := m.Called(ctx, exec, table, internalObjectIds, metadataFields)
	return args.Get(0).([]models.DataModelObject), args.Error(1)
}

func (m *IngestedDataReader) QueryIngestedObject(ctx context.Context, exec repositories.Executor,
	table models.Table, objectId string, metadataFields ...string,
) ([]models.DataModelObject, error) {
	args := m.Called(ctx, exec, table, objectId, metadataFields)
	return args.Get(0).([]models.DataModelObject), args.Error(1)
}

func (m *IngestedDataReader) QueryIngestedObjectByUniqueField(ctx context.Context,
	exec repositories.Executor, table models.Table, uniqueFieldValue string, uniqueFieldName string,
	metadataFields ...string,
) ([]models.DataModelObject, error) {
	args := m.Called(ctx, exec, table, uniqueFieldValue, uniqueFieldName, metadataFields)
	return args.Get(0).([]models.DataModelObject), args.Error(1)
}

func (m *IngestedDataReader) QueryAggregatedValue(ctx context.Context, exec repositories.Executor,
	tableName string, fieldName string, fieldType models.DataType, aggregator ast.Aggregator,
	filters []models.FilterWithType, options map[string]any,
) (any, error) {
	args := m.Called(ctx, exec, tableName, fieldName, fieldType, aggregator, filters, options)
	return args.Get(0), args.Error(1)
}

func (m *IngestedDataReader) ListIngestedObjects(ctx context.Context, exec repositories.Executor,
	table models.Table, params models.ExplorationOptions, cursorId *string, limit int, fieldsToRead ...string,
) ([]models.DataModelObject, error) {
	args := m.Called(ctx, exec, table, params, cursorId, limit, fieldsToRead)
	return args.Get(0).([]models.DataModelObject), args.Error(1)
}

func (m *IngestedDataReader) GatherFieldStatistics(ctx context.Context, exec repositories.Executor,
	table models.Table, orgId uuid.UUID,
) ([]models.FieldStatistics, error) {
	args := m.Called(ctx, exec, table, orgId)
	return args.Get(0).([]models.FieldStatistics), args.Error(1)
}
