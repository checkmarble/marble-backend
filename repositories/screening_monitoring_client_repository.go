package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const TABLE_INTERNAL_SCREENING_MONITORING = "_screening_monitoring_objects"

func tableNameWithPrefix(tableName string) string {
	return fmt.Sprintf("%s_%s", TABLE_INTERNAL_SCREENING_MONITORING, tableName)
}

func sanitizedTableName(exec Executor, tableName string) string {
	return pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, tableNameWithPrefix(tableName)})
}

// Table schema
// id: UUID, primary key UUID V7
// object_id: TEXT, foreign key to client_tables.object_id
// config_id: UUID, foreign key to screening_monitoring_configs.id
// created_at: TIMESTAMP WITH TIME ZONE
func (repo *ClientDbRepository) CreateInternalScreeningMonitoringTable(ctx context.Context, exec Executor, tableName string) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID NOT NULL PRIMARY KEY,
			object_id TEXT NOT NULL,
			config_id UUID NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
	`, sanitizedTableName(exec, tableName))

	_, err := exec.Exec(ctx, sql)
	return err
}

// Create index needed for the table
// uniq_idx_config_id_object_id: UNIQUE INDEX, config_id and object_id must be unique
// TODO: Should we add the index creation with the table creation?
func (repo *ClientDbRepository) CreateInternalScreeningMonitoringIndex(ctx context.Context, exec Executor, tableName string) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	// Unique index to have a unique object_id for a given config_id
	sql := fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS uniq_idx%s_config_id_object_id ON %s (config_id, object_id)",
		tableNameWithPrefix(tableName),
		sanitizedTableName(exec, tableName),
	)

	_, err := exec.Exec(ctx, sql)
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
		sanitizedTableName(exec, tableName),
	)

	_, err := exec.Exec(ctx, sql, uuid.Must(uuid.NewV7()), objectId, configId)
	return err
}
