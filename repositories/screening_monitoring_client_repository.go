package repositories

import (
	"context"
	"fmt"

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
// config_id: UUID, foreign key to screening_monitoring_configs.id
// created_at: TIMESTAMP WITH TIME ZONE
// Truncate the table name and the uniq index name to the maximum length of 63 characters
// Add unique index to have a unique object_id for a given config_id
func (repo *ClientDbRepository) CreateInternalScreeningMonitoringTable(ctx context.Context, exec Executor, tableName string) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	tableNameWithPrefix := tableNameWithPrefix(tableName)
	truncatedTableName := utils.TruncateIdentifier(tableNameWithPrefix)

	sanitizedTableName := sanitizedTableName(exec, truncatedTableName)
	truncatedUniqIndexName := utils.TruncateIdentifier(
		fmt.Sprintf(
			"_uniq_idx_config_id_object_id_%s",
			tableNameWithPrefix,
		),
	)

	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID NOT NULL PRIMARY KEY,
			object_id TEXT NOT NULL,
			config_id UUID NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
	`, sanitizedTableName)
	_, err := exec.Exec(ctx, sql)
	if err != nil {
		return err
	}

	// Unique index to have a unique object_id for a given config_id
	sql = fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (config_id, object_id)",
		truncatedUniqIndexName,
		sanitizedTableName,
	)
	_, err = exec.Exec(ctx, sql)

	return err
}

func (repo *ClientDbRepository) InsertScreeningMonitoringObject(ctx context.Context, exec Executor,
	tableName string, objectId string, configId uuid.UUID,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf(
		"INSERT INTO %s (id, object_id, config_id) VALUES ($1, $2, $3)",
		sanitizedTableName(exec, utils.TruncateIdentifier(tableNameWithPrefix(tableName))),
	)

	_, err := exec.Exec(ctx, sql, uuid.Must(uuid.NewV7()), objectId, configId)
	return err
}
