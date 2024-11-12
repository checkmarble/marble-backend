package repositories

import (
	"context"
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

func (repo *IngestionRepositoryImpl) batchInsertPayloads(ctx context.Context, exec Executor, payloads []models.ClientObject, table models.Table,
) error {
	columnNames := models.ColumnNames(table)
	query := NewQueryBuilder().Insert(tableNameWithSchema(exec, table.Name))

	for _, payload := range payloads {

		insertValues := generateInsertValues(payload, columnNames)
		// Add UUID to the insert values for the "id" field
		insertValues = append(insertValues, uuid.NewString())
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
