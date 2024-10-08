package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/checkmarble/marble-backend/models"
)

type IngestionRepository interface {
	IngestObjects(ctx context.Context, tx Transaction, payloads []models.ClientObject, table models.Table) (int, error)
}

type IngestionRepositoryImpl struct{}

func (repo *IngestionRepositoryImpl) IngestObjects(
	ctx context.Context,
	tx Transaction,
	payloads []models.ClientObject,
	table models.Table,
) (int, error) {
	if err := validateClientDbExecutor(tx); err != nil {
		return 0, err
	}

	mostRecentObjectIds, mostRecentPayloads := repo.mostRecentPayloadsByObjectId(payloads)

	previouslyIngestedObjects, err := repo.loadPreviouslyIngestedObjects(ctx, tx, mostRecentObjectIds, table.Name)
	if err != nil {
		return 0, err
	}

	payloadsToInsert, obsoleteIngestedObjectIds := repo.comparePayloadsToIngestedObjects(
		mostRecentPayloads,
		previouslyIngestedObjects,
	)

	if len(obsoleteIngestedObjectIds) > 0 {
		err := repo.batchUpdateValidUntilOnObsoleteObjects(
			ctx,
			tx,
			table.Name,
			obsoleteIngestedObjectIds,
		)
		if err != nil {
			return 0, err
		}
	}

	if len(payloadsToInsert) > 0 {
		if err := repo.batchInsertPayloadsAndEnumValues(ctx, tx, payloadsToInsert, table); err != nil {
			return 0, err
		}
	}

	return len(payloadsToInsert), nil
}

func objectIdAndUpdatedAtFromPayload(payload models.ClientObject) (string, time.Time) {
	objectIdItf := payload.Data["object_id"]
	updatedAtItf := payload.Data["updated_at"]
	objectId := objectIdItf.(string)
	updatedAt := updatedAtItf.(time.Time)

	return objectId, updatedAt
}

func (repo *IngestionRepositoryImpl) mostRecentPayloadsByObjectId(payloads []models.ClientObject) ([]string, []models.ClientObject) {
	recentMap := make(map[string]models.ClientObject)
	for _, payload := range payloads {
		objectId, updatedAt := objectIdAndUpdatedAtFromPayload(payload)

		if seen, ok := recentMap[objectId]; ok {
			_, seenUpdatedAt := objectIdAndUpdatedAtFromPayload(seen)
			if updatedAt.After(seenUpdatedAt) {
				recentMap[objectId] = payload
			}
		} else {
			recentMap[objectId] = payload
		}
	}

	mostRecentPayloads := make([]models.ClientObject, 0, len(recentMap))
	mostRecentObjectIds := make([]string, 0, len(recentMap))
	for key, obj := range recentMap {
		mostRecentObjectIds = append(mostRecentObjectIds, key)
		mostRecentPayloads = append(mostRecentPayloads, obj)
	}

	return mostRecentObjectIds, mostRecentPayloads
}

type DBObject struct {
	Id        string    `db:"id"`
	ObjectId  string    `db:"object_id"`
	UpdatedAt time.Time `db:"updated_at"`
}
type IngestedObject struct {
	Id        string
	ObjectId  string
	UpdatedAt time.Time
}

func (repo *IngestionRepositoryImpl) loadPreviouslyIngestedObjects(ctx context.Context,
	exec Executor, objectIds []string, tableName string,
) ([]IngestedObject, error) {
	query := NewQueryBuilder().
		Select("id, object_id, updated_at").
		From(tableNameWithSchema(exec, tableName)).
		Where(squirrel.Eq{"object_id": objectIds}).
		Where(squirrel.Eq{"valid_until": "Infinity"})

	return SqlToListOfModels(ctx, exec, query, func(db DBObject) (IngestedObject, error) { return IngestedObject(db), nil })
}

func (repo *IngestionRepositoryImpl) comparePayloadsToIngestedObjects(
	payloads []models.ClientObject, previouslyIngestedObjects []IngestedObject,
) ([]models.ClientObject, []string) {
	previouslyIngestedMap := make(map[string]IngestedObject)
	for _, obj := range previouslyIngestedObjects {
		previouslyIngestedMap[obj.ObjectId] = obj
	}

	payloadsToInsert := make([]models.ClientObject, 0, len(payloads))
	obsoleteIngestedObjectIds := make([]string, 0, len(previouslyIngestedMap))

	for _, payload := range payloads {
		objectId, updatedAt := objectIdAndUpdatedAtFromPayload(payload)

		existingObject, exists := previouslyIngestedMap[objectId]
		if !exists {
			payloadsToInsert = append(payloadsToInsert, payload)
		} else if updatedAt.After(existingObject.UpdatedAt) ||
			updatedAt.Equal(existingObject.UpdatedAt) {
			payloadsToInsert = append(payloadsToInsert, payload)
			obsoleteIngestedObjectIds = append(obsoleteIngestedObjectIds, existingObject.Id)
		}
	}

	return payloadsToInsert, obsoleteIngestedObjectIds
}

func (repo *IngestionRepositoryImpl) batchUpdateValidUntilOnObsoleteObjects(ctx context.Context,
	exec Executor, tableName string, obsoleteIngestedObjectIds []string,
) error {
	sql := NewQueryBuilder().
		Update(tableNameWithSchema(exec, tableName)).
		Set("valid_until", "now()").
		Where(squirrel.Eq{"id": obsoleteIngestedObjectIds})
	err := ExecBuilder(ctx, exec, sql)

	return err
}

func (repo *IngestionRepositoryImpl) batchInsertPayloadsAndEnumValues(ctx context.Context,
	exec Executor, payloads []models.ClientObject, table models.Table,
) error {
	columnNames := models.ColumnNames(table)
	query := NewQueryBuilder().Insert(tableNameWithSchema(exec, table.Name))

	enumValues := buildEnumValuesWithEnumFields(table)

	for _, payload := range payloads {
		collectEnumValues(payload, enumValues)

		insertValues := generateInsertValues(payload, columnNames)
		// Add UUID to the insert values for the "id" field
		insertValues = append(insertValues, uuid.NewString())
		query = query.Values(insertValues...)
	}

	err := batchInsertEnumValues(ctx, exec, enumValues, table)
	if err != nil {
		return fmt.Errorf("batchInsertEnumValues error: %w", err)
	}

	columnNames = append(columnNames, "id")
	query = query.Columns(columnNames...)

	err = ExecBuilder(ctx, exec, query)
	if IsUniqueViolationError(err) {
		return errors.Wrap(models.ConflictError, "unique constraint violation during ingestion")
	}

	return err
}

type EnumValues map[string]map[any]bool

func buildEnumValuesWithEnumFields(table models.Table) EnumValues {
	enumValues := make(EnumValues)
	for fieldName := range table.Fields {
		dataType := table.Fields[fieldName].DataType
		if table.Fields[fieldName].IsEnum && (dataType == models.String || dataType == models.Float) {
			enumValues[fieldName] = make(map[any]bool)
		}
	}
	return enumValues
}

// mutates enumValues
func collectEnumValues(payload models.ClientObject, enumValues EnumValues) {
	for fieldName := range enumValues {
		value := payload.Data[fieldName]
		if value != nil && value != "" {
			enumValues[fieldName][value] = true
		}
	}
}

func generateInsertValues(payload models.ClientObject, columnNames []string) []any {
	insertValues := make([]any, len(columnNames))
	for i, fieldName := range columnNames {
		insertValues[i] = payload.Data[fieldName]
	}
	return insertValues
}

// This has to be done in 2 queries because there cannot be multiple ON CONFLICT clauses per query
func batchInsertEnumValues(ctx context.Context, exec Executor, enumValues EnumValues, table models.Table) error {
	textQuery := NewQueryBuilder().
		Insert("data_model_enum_values").
		Columns("field_id", "text_value").
		Suffix("ON CONFLICT ON CONSTRAINT unique_data_model_enum_text_values_field_id_value DO NOTHING")

	floatQuery := NewQueryBuilder().
		Insert("data_model_enum_values").
		Columns("field_id", "float_value").
		Suffix("ON CONFLICT ON CONSTRAINT unique_data_model_enum_float_values_field_id_value DO NOTHING")

	// Hack to avoid empty query, which would cause an execution error
	var shouldInsertTextValues bool
	var shouldInsertFloatValues bool

	for fieldName, values := range enumValues {
		fieldId := table.Fields[fieldName].ID
		dataType := table.Fields[fieldName].DataType

		for value := range values {
			if dataType == models.String {
				textQuery = textQuery.Values(fieldId, value)
				shouldInsertTextValues = true
			} else if dataType == models.Float {
				floatQuery = floatQuery.Values(fieldId, value)
				shouldInsertFloatValues = true
			}
		}
	}

	if shouldInsertTextValues {
		err := ExecBuilder(ctx, exec, textQuery)
		if err != nil {
			return err
		}
	}
	if shouldInsertFloatValues {
		err := ExecBuilder(ctx, exec, floatQuery)
		if err != nil {
			return err
		}
	}

	return nil
}
