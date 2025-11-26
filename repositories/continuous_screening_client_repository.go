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
// Add unique index to ensure a unique combination of (config_stable_id, object_type, object_id)
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

	// Unique index to have a unique (object_type, object_id) combination for a given config_stable_id
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

// Table schema
// id: UUID, primary key
// object_type: TEXT, type of the object
// object_id: TEXT
// config_stable_id: UUID, foreign key to continuous_screening_configs.stable_id
// action: TEXT, "add" or "remove"
// user_id: UUID NULLABLE, id of the user who performed the action
// api_key_id: UUID NULLABLE, id of the api key who performed the action
// created_at: TIMESTAMP WITH TIME ZONE
// extra: JSONB, extra information
//
// Indexes:
//   - idx_monitored_objects_audit_obj: History of a specific object (e.g. "Show me history for Company:123")
//   - idx_monitored_objects_audit_user: User Activity (e.g. "What did User X do?")
//   - idx_monitored_objects_audit_api_key: API Key Activity
//   - idx_monitored_objects_audit_config: Config Usage (e.g. "Activity for this specific configuration")
//
// CreateInternalContinuousScreeningAuditTable creates the audit table for monitored objects
// It is a single table for all object types, containing the history of actions (add/remove)
func (repo *ClientDbRepository) CreateInternalContinuousScreeningAuditTable(ctx context.Context, exec Executor) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	tableName := sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_AUDIT)

	// 1. Create the Table
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id UUID NOT NULL PRIMARY KEY,
			object_type TEXT NOT NULL,
			object_id TEXT NOT NULL,
			config_stable_id UUID NOT NULL,
			action TEXT NOT NULL,
			user_id UUID,
			api_key_id UUID,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			extra JSONB
		);
	`, tableName)
	_, err := exec.Exec(ctx, sql)
	if err != nil {
		return err
	}

	// 2. Create Indexes
	// Index: History of a specific object (e.g. "Show me history for Company:123")
	// We include created_at DESC to get the latest actions first
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS idx_monitored_objects_audit_obj ON %s (object_type, object_id, created_at DESC)",
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index: User Activity (e.g. "What did User X do?")
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS idx_monitored_objects_audit_user ON %s (user_id, created_at DESC) WHERE user_id IS NOT NULL",
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index: API Key Activity
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS idx_monitored_objects_audit_api_key ON %s (api_key_id, created_at DESC) WHERE api_key_id IS NOT NULL",
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index: Config Usage (e.g. "Activity for this specific configuration")
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS idx_monitored_objects_audit_config ON %s (config_stable_id, created_at DESC)",
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	return nil
}

func (repo *ClientDbRepository) InsertContinuousScreeningAudit(
	ctx context.Context,
	exec Executor,
	audit models.CreateContinuousScreeningAudit,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf(`
		INSERT INTO %s 
		(id, object_type, object_id, config_stable_id, action, user_id, api_key_id, extra) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_AUDIT))

	_, err := exec.Exec(ctx, sql,
		uuid.Must(uuid.NewV7()),
		audit.ObjectType,
		audit.ObjectId,
		audit.ConfigStableId,
		audit.Action.String(),
		audit.UserId,
		audit.ApiKeyId,
		audit.Extra,
	)
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
