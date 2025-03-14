package repositories

import (
	"context"
	"fmt"
	"slices"
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

	mostRecentObjectIds, mostRecentPayloads := mostRecentPayloadsByObjectId(payloads)

	fieldsToLoad := fieldsToLoadFromDb(payloads)
	previouslyIngestedObjects, err := repo.loadPreviouslyIngestedObjects(ctx, tx,
		mostRecentObjectIds, table, fieldsToLoad)
	if err != nil {
		return 0, err
	}

	payloadsToInsert, obsoleteIngestedObjectIds, validationErrors := compareAndMergePayloadsWithIngestedObjects(
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

// Try to only load the fields that are actually missing from the payloads (in the case of a partial update).
// The same method is used for the POST and PATCH endpoints, but if no fields are missing then there is a
// covering index on the object_id and updated_at columns that can be used.
// This makes the POST endpoint possibly faster to respond than the PATCH endpoint for ingestion (when there
// is data present and some payload have missing fields).
func fieldsToLoadFromDb(payloads []models.ClientObject) []string {
	missingFields := make(map[string]struct{})
	missingFields["object_id"] = struct{}{}
	missingFields["updated_at"] = struct{}{}
	for _, payload := range payloads {
		for _, field := range payload.MissingFieldsToLookup {
			missingFields[field.Field.Name] = struct{}{}
		}
	}
	missingFieldsList := make([]string, 0, len(missingFields))
	for field := range missingFields {
		missingFieldsList = append(missingFieldsList, field)
	}
	return missingFieldsList
}

func objectIdAndUpdatedAtFromPayload(payload models.ClientObject) (string, time.Time) {
	objectIdItf := payload.Data["object_id"]
	updatedAtItf := payload.Data["updated_at"]
	objectId := objectIdItf.(string)
	updatedAt := updatedAtItf.(time.Time)

	return objectId, updatedAt
}

// Keep only the most recent (as per updated_at) payload for each object_id. In case of equal values seen, the first one wins.
// The returned slices are sorted by object_id for unit tests & query plan stability.
func mostRecentPayloadsByObjectId(payloads []models.ClientObject) ([]string, []models.ClientObject) {
	idxToKeep := make(map[string]int, len(payloads))
	for i, payload := range payloads {
		objectId, updatedAt := objectIdAndUpdatedAtFromPayload(payload)

		previousIdForThisObject, ok := idxToKeep[objectId]
		if !ok {
			idxToKeep[objectId] = i
		} else {
			previousUpdatedAt := payloads[previousIdForThisObject].Data["updated_at"].(time.Time)
			if updatedAt.After(previousUpdatedAt) {
				idxToKeep[objectId] = i
			}
		}
	}

	// Collect and sort object IDs
	objectIds := make([]string, 0, len(idxToKeep))
	for objectId := range idxToKeep {
		objectIds = append(objectIds, objectId)
	}
	slices.Sort(objectIds) // Sort the object IDs

	// Construct result slices using sorted object IDs
	mostRecentPayloads := make([]models.ClientObject, 0, len(objectIds))
	mostRecentObjectIds := make([]string, 0, len(objectIds))
	for _, objectId := range objectIds {
		idx := idxToKeep[objectId]
		mostRecentPayloads = append(mostRecentPayloads, payloads[idx])
		mostRecentObjectIds = append(mostRecentObjectIds, objectId)
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
	fieldsToLoad []string,
) ([]ingestedObject, error) {
	// Should be sorted consistently for unit tests & query plan stability
	slices.Sort(fieldsToLoad)
	fieldsToLoad = append(fieldsToLoad, "id")
	qualifiedTableName := tableNameWithSchema(tx, table.Name)

	q := NewQueryBuilder().
		Select(fieldsToLoad...).
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
		for i, columnName := range fieldsToLoad {
			objectAsMap[columnName] = values[i]
		}
		id, ok := objectAsMap["id"].([16]byte)
		if !ok {
			return nil, fmt.Errorf("error while converting ID to UUID")
		}
		output = append(output, ingestedObject{
			id:        uuid.UUID(id).String(),
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
// The "previouslyIngestedObjects" slice contains objects where the "data" map only contains the fields that were loaded from the DB,
// not necessarily all the fields that are present in the data model.
func compareAndMergePayloadsWithIngestedObjects(
	payloads []models.ClientObject,
	previouslyIngestedObjects []ingestedObject,
) ([]models.ClientObject, []string, models.IngestionValidationErrors) {
	previouslyIngestedMap := make(map[string]ingestedObject)
	for _, obj := range previouslyIngestedObjects {
		previouslyIngestedMap[obj.objectId] = obj
	}

	payloadsToInsert := make([]models.ClientObject, 0, len(payloads))
	obsoleteIngestedObjectIds := make([]string, 0, len(previouslyIngestedMap))
	validationErrors := make(models.IngestionValidationErrors, len(payloads))

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
