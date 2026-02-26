package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func sanitizedTableName(exec Executor, tableName string) string {
	return pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, tableName})
}

func (repo *ClientDbRepository) IsContinuousScreeningSetup(ctx context.Context, exec Executor) (bool, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return false, err
	}

	sql := `select exists(select 1 from information_schema.tables where table_name = '_monitored_objects')`
	row := exec.QueryRow(ctx, sql)

	var exists bool

	if err := row.Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

// Table schema
// id: UUID, primary key
// object_type: TEXT
// object_id: TEXT
// config_stable_id: UUID
// created_at: TIMESTAMP WITH TIME ZONE
//
// Indexes:
//   - internal_uniq_idx_monitored_objects_config_type_id: Unique index on (config_stable_id, object_type, object_id)
//   - internal_idx_monitored_objects_type_id: Index on (object_type, object_id)
//   - internal_idx_monitored_objects_config: Index on (config_stable_id, created_at DESC, id DESC)
//   - internal_idx_monitored_objects_type: Index on (object_type, created_at DESC, id DESC)
//
// ⚠️ Careful about the indexes prefixes to avoid deletion by IndexDeletionWorker (see: IndexDeletionWorker)
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
	uniqIndexName := "internal_uniq_idx_monitored_objects_config_type_id"
	sql = fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (config_stable_id, object_type, object_id)",
		uniqIndexName,
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index on object_type and object_id
	indexName := "internal_idx_monitored_objects_type_id"
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s (object_type, object_id)",
		indexName,
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index for list queries with filtering and pagination (created_at DESC for ordering)
	// Index for filtering by config_stable_id
	configIndexName := "internal_idx_monitored_objects_config"
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s (config_stable_id, created_at DESC, id DESC)",
		configIndexName,
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index for filtering by object_type
	typeIndexName := "internal_idx_monitored_objects_type"
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS %s ON %s (object_type, created_at DESC, id DESC)",
		typeIndexName,
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
	ignoreConflicts bool,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := "INSERT INTO %s (id, object_type, object_id, config_stable_id) VALUES ($1, $2, $3, $4)"

	if ignoreConflicts {
		sql += " ON CONFLICT DO NOTHING"
	}

	query := fmt.Sprintf(
		sql,
		sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS),
	)

	_, err := exec.Exec(ctx, query, uuid.Must(uuid.NewV7()), objectType, objectId, configStableId)
	return err
}

// Table schema
// id: UUID, primary key
// object_type: TEXT
// object_id: TEXT
// config_stable_id: UUID
// action: TEXT, "add" or "remove"
// user_id: UUID NULLABLE, id of the user who performed the action
// api_key_id: UUID NULLABLE, id of the api key who performed the action
// created_at: TIMESTAMP WITH TIME ZONE
// extra: JSONB, extra information
//
// Indexes:
//   - internal_idx_monitored_objects_audit_obj: History of a specific object (e.g. "Show me history for Company:123")
//   - internal_idx_monitored_objects_audit_user: User Activity (e.g. "What did User X do?")
//   - internal_idx_monitored_objects_audit_api_key: API Key Activity
//   - internal_idx_monitored_objects_audit_config: Config Usage (e.g. "Activity for this specific configuration")
//
// ⚠️ Careful about the indexes prefixes to avoid deletion by IndexDeletionWorker (see: IndexDeletionWorker)
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
		"CREATE INDEX IF NOT EXISTS internal_idx_monitored_objects_audit_obj ON %s (object_type, object_id, created_at DESC)",
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index: User Activity (e.g. "What did User X do?")
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS internal_idx_monitored_objects_audit_user ON %s (user_id, created_at DESC) WHERE user_id IS NOT NULL",
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index: API Key Activity
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS internal_idx_monitored_objects_audit_api_key ON %s (api_key_id, created_at DESC) WHERE api_key_id IS NOT NULL",
		tableName,
	)
	if _, err := exec.Exec(ctx, sql); err != nil {
		return err
	}

	// Index: Config Usage (e.g. "Activity for this specific configuration")
	sql = fmt.Sprintf(
		"CREATE INDEX IF NOT EXISTS internal_idx_monitored_objects_audit_config ON %s (config_stable_id, created_at DESC)",
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

func (repo *ClientDbRepository) DeleteContinuousScreeningObject(
	ctx context.Context,
	exec Executor,
	input models.DeleteContinuousScreeningObject,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Delete(sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS)).
		Where(squirrel.Eq{"object_type": input.ObjectType}).
		Where(squirrel.Eq{"object_id": input.ObjectId}).
		Where(squirrel.Eq{"config_stable_id": input.ConfigStableId})

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	tag, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.Wrap(models.NotFoundError, "object not found")
	}

	return nil
}

func (repo *ClientDbRepository) ListMonitoredObjects(
	ctx context.Context,
	exec Executor,
	filters models.ListMonitoredObjectsFilters,
	pagination models.PaginationAndSorting,
) ([]models.ContinuousScreeningMonitoredObject, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	tableName := sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS)

	if pagination.Sorting != models.SortingFieldCreatedAt {
		return nil, errors.Wrapf(models.BadParameterError, "invalid sorting field: %s", pagination.Sorting)
	}

	orderCond := fmt.Sprintf("%s %s, id %s", pagination.Sorting, pagination.Order, pagination.Order)

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningMonitoredObjectColumn...).
		From(tableName).
		OrderBy(orderCond).
		Limit(uint64(pagination.Limit))

	// Apply filters
	if len(filters.ConfigStableIds) > 0 {
		query = query.Where(squirrel.Eq{"config_stable_id": filters.ConfigStableIds})
	}
	if len(filters.ObjectTypes) > 0 {
		query = query.Where(squirrel.Eq{"object_type": filters.ObjectTypes})
	}
	if len(filters.ObjectIds) > 0 {
		query = query.Where(squirrel.Eq{"object_id": filters.ObjectIds})
	}
	if filters.StartDate != nil {
		query = query.Where(squirrel.GtOrEq{"created_at": *filters.StartDate})
	}
	if filters.EndDate != nil {
		query = query.Where(squirrel.LtOrEq{"created_at": *filters.EndDate})
	}

	if pagination.OffsetId != "" {
		offsetId, err := uuid.Parse(pagination.OffsetId)
		if err != nil {
			return nil, errors.Wrap(models.BadParameterError, "invalid offset_id")
		}
		offsetObject, err := repo.GetMonitoredObject(ctx, exec, offsetId)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.Wrap(models.NotFoundError,
					"No row found matching the provided offset_id")
			}
			return nil, errors.Wrap(err, "error fetching offset monitored object")
		}
		if pagination.Order == models.SortingOrderDesc {
			query = query.Where(
				squirrel.Expr("(created_at, id) < (?, ?)", offsetObject.CreatedAt, offsetObject.Id),
			)
		} else {
			query = query.Where(
				squirrel.Expr("(created_at, id) > (?, ?)", offsetObject.CreatedAt, offsetObject.Id),
			)
		}
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningMonitoredObject)
}

func (repo *ClientDbRepository) CountMonitoredObjectsByConfigStableIds(
	ctx context.Context,
	exec Executor,
	configStableIds []uuid.UUID,
) (int, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}

	// If no config stable IDs provided, return 0
	if len(configStableIds) == 0 {
		return 0, nil
	}

	query := NewQueryBuilder().
		Select("count(*) as count").
		From(sanitizedTableName(exec, dbmodels.TABLE_CONTINUOUS_SCREENING_MONITORED_OBJECTS)).
		Where(squirrel.Eq{"config_stable_id": configStableIds})

	var count int
	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	err = exec.QueryRow(ctx, sql, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
