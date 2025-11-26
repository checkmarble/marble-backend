package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func sanitizedTableName(exec Executor, tableName string) string {
	return pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, tableName})
}

// Table schema
// id: UUID, primary key
// object_type: TEXT
// object_id: TEXT
// config_stable_id: UUID
// created_at: TIMESTAMP WITH TIME ZONE
// Add unique index to have a unique object_id for a given config_id
func (repo *ClientDbRepository) CreateInternalContinuousScreeningTable(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	tableName := sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS)

	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID NOT NULL PRIMARY KEY,
			object_type TEXT NOT NULL,
			object_id TEXT NOT NULL,
			config_stable_id UUID NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
	`, tableName)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Unique index to have a unique object_id for a given config_id
	uniqIndexName := fmt.Sprintf(
		"uniq_idx_config_object_type_id%s",
		dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS,
	)
	sql = fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (config_stable_id, object_type, object_id)",
		uniqIndexName,
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index on object_type and object_id
	indexName := fmt.Sprintf(
		"idx_object_type_object_id%s",
		dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS,
	)
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s (object_type, object_id)",
		indexName,
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	return nil
}

func (repo *ClientDbRepository) InsertContinuousScreeningObject(
	ctx context.Context,
	exec Executor,
	objectType string,
	objectId string,
	configStableId uuid.UUID,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf(
		"INSERT INTO %s (id, object_type, object_id, config_stable_id) VALUES ($1, $2, $3, $4)",
		sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS),
	)

	_, err := exec.Exec(ctx, sql, uuid.Must(uuid.NewV7()), objectType, objectId, configStableId)
	return err
}

func (repo *ClientDbRepository) GetMonitoredObject(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
) (models.ContinuousScreeningMonitoredObject, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return models.ContinuousScreeningMonitoredObject{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningMonitoredObjectColumn...).
		From(sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS)).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningMonitoredObject)
}

// List monitored objects by object IDs
// This function is used to check if an object is under continuous screening
// If not, the result will not contain the Monitored object model for the object ID
// Example:
//   - Monitored object: ["object_id_1", "object_id_2"]
//   - Not monitored object: ["object_id_3", "object_id_4"]
//   - objectIds: ["object_id_1", "object_id_2", "object_id_3", "object_id_4"]
//   - Result: ["object_id_1", "object_id_2"]
func (repo *ClientDbRepository) ListMonitoredObjectsByObjectIds(
	ctx context.Context,
	exec Executor,
	objectType string,
	objectIds []string,
) ([]models.ContinuousScreeningMonitoredObject, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningMonitoredObjectColumn...).
		From(sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS)).
		Where(squirrel.Eq{"object_type": objectType}).
		Where(squirrel.Eq{"object_id": objectIds})

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningMonitoredObject)
}
