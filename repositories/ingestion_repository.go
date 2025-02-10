package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

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

	previouslyIngestedObjects, err := repo.loadPreviouslyIngestedObjects(ctx, tx, mostRecentObjectIds, table)
	if err != nil {
		return 0, err
	}

	payloadsToInsert, obsoleteIngestedObjectIds, validationErrors := repo.comparePayloadsToIngestedObjects(
		mostRecentPayloads,
		previouslyIngestedObjects,
	)
	if len(validationErrors) > 0 {
		return 0, errors.Join(models.BadParameterError, validationErrors)
	}

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
		if err := repo.batchInsertPayloads(ctx, tx, payloadsToInsert, table); err != nil {
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

type ingestedObject struct {
	id        string
	objectId  string
	updatedAt time.Time
	data      map[string]any
}

func (repo *IngestionRepositoryImpl) loadPreviouslyIngestedObjects(
	ctx context.Context,
	tx Transaction,
	objectIds []string,
	table models.Table,
) ([]ingestedObject, error) {
	columnNames := models.ColumnNames(table)
	columnNames = append(columnNames, "id")
	qualifiedTableName := tableNameWithSchema(tx, table.Name)

	q := NewQueryBuilder().
		Select(columnNames...).
		From(qualifiedTableName).
		Where(rowIsValid(qualifiedTableName)).
		Where(squirrel.Eq{"object_id": objectIds})

	sql, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("error while querying DB: %w", err)
	}
	defer rows.Close()
	output := make([]ingestedObject, 0, len(objectIds))
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("error while fetching rows: %w", err)
		}

		objectAsMap := make(map[string]any)
		for i, columnName := range columnNames {
			objectAsMap[columnName] = values[i]
		}
		output = append(output, ingestedObject{
			id:        objectAsMap["id"].(string),
			objectId:  objectAsMap["object_id"].(string),
			updatedAt: objectAsMap["updated_at"].(time.Time),
			data:      objectAsMap,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while iterating over rows: %w", err)
	}

	return output, nil
}

// Takes a list of payloads and a list of previously ingested objects. The payloads may optionally include
// a list of fields that should be checked for existence in the ingested objects.
// - a list of complete payloads that should be inserted, obtained by merging the payloads with the missing fields from the previously ingested objects
// - a list of IDs of objects that should be marked as obsolete
// - a map of validation errors for each payload (if any required missing fields are also missing in the ingested objects)
func (repo *IngestionRepositoryImpl) comparePayloadsToIngestedObjects(
	payloads []models.ClientObject,
	previouslyIngestedObjects []ingestedObject,
) ([]models.ClientObject, []string, models.IngestionValidationErrorsMultiple) {
	previouslyIngestedMap := make(map[string]ingestedObject)
	for _, obj := range previouslyIngestedObjects {
		previouslyIngestedMap[obj.objectId] = obj
	}

	payloadsToInsert := make([]models.ClientObject, 0, len(payloads))
	obsoleteIngestedObjectIds := make([]string, 0, len(previouslyIngestedMap))
	validationErrors := make(models.IngestionValidationErrorsMultiple, len(payloads))

	for _, payload := range payloads {
		objectId, updatedAt := objectIdAndUpdatedAtFromPayload(payload)

		existingObject, exists := previouslyIngestedMap[objectId]
		for _, field := range payload.MissingFieldsToLookup {
			foundInPreviousVersion := exists && existingObject.data[field.Field.Name] != nil
			if !field.Field.Nullable && !foundInPreviousVersion {
				// add to the returned errors if the field that was missing in the payload is required and
				// is also missing in the previously ingested version
				if validationErrors[objectId] == nil {
					validationErrors[objectId] = make(map[string]string, len(payload.MissingFieldsToLookup))
				}
				validationErrors[objectId][field.Field.Name] = field.ErrorIfMissing
			}
			// In any case, add the field to the payload
			payload.Data[field.Field.Name] = existingObject.data[field.Field.Name]
		}

		if !exists {
			payloadsToInsert = append(payloadsToInsert, payload)
		} else if updatedAt.After(existingObject.updatedAt) ||
			updatedAt.Equal(existingObject.updatedAt) {
			payloadsToInsert = append(payloadsToInsert, payload)
			obsoleteIngestedObjectIds = append(obsoleteIngestedObjectIds, existingObject.id)
		}
	}

	return payloadsToInsert, obsoleteIngestedObjectIds, validationErrors
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

func (repo *IngestionRepositoryImpl) batchInsertPayloads(ctx context.Context, exec Executor, payloads []models.ClientObject, table models.Table,
) error {
	columnNames := models.ColumnNames(table)
	query := NewQueryBuilder().Insert(tableNameWithSchema(exec, table.Name))

	for _, payload := range payloads {

		insertValues := generateInsertValues(payload, columnNames)
		// Add UUID to the insert values for the "id" field
		insertValues = append(insertValues, uuid.Must(uuid.NewV7()).String())
		query = query.Values(insertValues...)
	}

	columnNames = append(columnNames, "id")
	query = query.Columns(columnNames...)

	err := ExecBuilder(ctx, exec, query)
	if IsUniqueViolationError(err) {
		return errors.Wrap(models.ConflictError, "unique constraint violation during ingestion")
	}

	return err
}

func generateInsertValues(payload models.ClientObject, columnNames []string) []any {
	insertValues := make([]any, len(columnNames))
	for i, fieldName := range columnNames {
		insertValues[i] = payload.Data[fieldName]
	}
	return insertValues
}
