package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const TABLE_INTERNAL_MONITORED_OBJECTS = "_monitored_objects"

func tableNameWithPrefix(tableName string) string {
	return fmt.Sprintf("%s_%s", TABLE_INTERNAL_MONITORED_OBJECTS, tableName)
}

func sanitizedTableName(exec Executor, tableName string) string {
	return pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, tableName})
}

// Table schema
// id: UUID, primary key
// object_id: TEXT, foreign key to client_tables.object_id
// config_stable_id: UUID, foreign key to continuous_screening_configs.stable_id
// created_at: TIMESTAMP WITH TIME ZONE
// Truncate the table name and the uniq index name to the maximum length of 63 characters
// Add unique index to have a unique object_id for a given config_id
func (repo *ClientDbRepository) CreateInternalContinuousScreeningTable(ctx context.Context, exec Executor, tableName string) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	tableNameWithPrefix := tableNameWithPrefix(tableName)
	truncatedTableName := utils.TruncateIdentifier(tableNameWithPrefix)

	sanitizedTableName := sanitizedTableName(exec, truncatedTableName)
	truncatedUniqIndexName := utils.TruncateIdentifier(
		fmt.Sprintf(
			"_uniq_idx_config_stable_id_object_id_%s",
			tableNameWithPrefix,
		),
	)

	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID NOT NULL PRIMARY KEY,
			object_id TEXT NOT NULL,
			config_stable_id UUID NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
	`, sanitizedTableName)
	_, err := exec.Exec(ctx, sql)
	if err != nil {
		return err
	}

	// Unique index to have a unique object_id for a given config_id
	sql = fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (config_stable_id, object_id)",
		truncatedUniqIndexName,
		sanitizedTableName,
	)
	_, err = exec.Exec(ctx, sql)

	return err
}

func (repo *ClientDbRepository) InsertContinuousScreeningObject(
	ctx context.Context,
	exec Executor,
	tableName string,
	objectId string,
	configStableId uuid.UUID,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf(
		"INSERT INTO %s (id, object_id, config_stable_id) VALUES ($1, $2, $3)",
		sanitizedTableName(exec, utils.TruncateIdentifier(tableNameWithPrefix(tableName))),
	)

	_, err := exec.Exec(ctx, sql, uuid.Must(uuid.NewV7()), objectId, configStableId)
	return err
}

func (repo *ClientDbRepository) GetMonitoredObject(
	ctx context.Context,
	exec Executor,
	objectType string,
	id uuid.UUID,
) (models.ContinuousScreeningMonitoredObject, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return models.ContinuousScreeningMonitoredObject{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningMonitoredObjectColumn...).
		From(sanitizedTableName(exec, utils.TruncateIdentifier(tableNameWithPrefix(objectType)))).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningMonitoredObject)
}

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
		From(sanitizedTableName(exec, utils.TruncateIdentifier(tableNameWithPrefix(objectType)))).
		Where(squirrel.Eq{"object_id": objectIds})

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningMonitoredObject)
}
