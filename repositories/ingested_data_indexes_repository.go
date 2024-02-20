package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/pg_indexes"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

var INDEX_CREATION_TIMEOUT time.Duration = 60 * 4 // 4 hours

func (repo *ClientDbRepository) ListAllValidIndexes(
	ctx context.Context,
	exec Executor,
) ([]models.ConcreteIndex, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	pgIndexes, err := repo.listAllIndexes(ctx, exec)
	if err != nil {
		return nil, errors.Wrap(err, "error while listing all indexes")
	}

	var validOrPendingIndexes []models.ConcreteIndex
	for _, pgIndex := range pgIndexes {
		if pgIndex.IsValid || pgIndex.CreationInProgress {
			validOrPendingIndexes = append(validOrPendingIndexes, pgIndex.AdaptConcreteIndex())
		}
	}

	return validOrPendingIndexes, nil
}

func (repo *ClientDbRepository) CountPendingIndexes(
	ctx context.Context,
	exec Executor,
) (int, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}

	pgIndexes, err := repo.listAllIndexes(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "error while listing all indexes")
	}

	count := 0
	for _, pgIndex := range pgIndexes {
		if pgIndex.CreationInProgress {
			count++
		}
	}
	return count, nil
}

func (repo *ClientDbRepository) listAllIndexes(
	ctx context.Context,
	exec Executor,
) ([]pg_indexes.PGIndex, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := `
	SELECT
		pg_get_indexdef(pg_class_idx.oid) AS indexdef,
		pg_class_idx.relname AS indexname,
		pgidx.indisvalid,
		pgidx.indexrelid,
		pg_class_table.relname AS tablename
	FROM pg_namespace AS pgn
	INNER JOIN pg_class AS pg_class_table ON (pgn.oid=pg_class_table.relnamespace)
	INNER JOIN pg_index AS pgidx ON (pgidx.indrelid=pg_class_table.oid)
	INNER JOIN pg_class AS pg_class_idx ON(pgidx.indexrelid=pg_class_idx.oid)
	WHERE nspname=$1
`
	rows, err := exec.Query(ctx, sql, exec.DatabaseSchema().Schema)
	if err != nil {
		return nil, errors.Wrap(err, "error while querying DB to read indexes")
	}
	pgIndexRows, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (pg_indexes.PGIndex, error) {
		var index pg_indexes.PGIndex
		err := row.Scan(&index.Definition, &index.Name, &index.IsValid, &index.RelationId, &index.TableName)
		return index, err
	})
	if err != nil {
		return nil, errors.Wrap(err, "error while collecting rows for indexes")
	}

	// Now read indexes that are currently being created
	rows, err = exec.Query(ctx, "SELECT index_relid FROM pg_stat_progress_create_index")
	if err != nil {
		return nil, errors.Wrap(err, "error while querying DB to read indexes in creation")
	}
	creationInProgressIdxOids, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (uint32, error) {
		var indexRelid uint32
		err := row.Scan(&indexRelid)
		return indexRelid, err
	})
	if err != nil {
		return nil, errors.Wrap(err, "error while collecting rows for indexes in creation")
	}

	// Now update the list of indexes with their "in creation" status
	for _, oid := range creationInProgressIdxOids {
		for i, idx := range pgIndexRows {
			if idx.RelationId == oid {
				pgIndexRows[i].CreationInProgress = true
			}
		}
	}

	return pgIndexRows, nil
}

func (repo *ClientDbRepository) CreateIndexesAsync(
	ctx context.Context,
	exec Executor,
	indexes []models.ConcreteIndex,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	go asynchronouslyCreateIndexes(ctx, exec, indexes)

	return nil
}

func asynchronouslyCreateIndexes(
	ctx context.Context,
	exec Executor,
	indexes []models.ConcreteIndex,
) {
	ctx = context.WithoutCancel(ctx)
	ctx, _ = context.WithTimeout(ctx, INDEX_CREATION_TIMEOUT*time.Minute)
	// The function is meant to be executed asynchronously and return way after the request was finished,
	// so we don't return any error
	// However the indexes are created one after the other to avoid a (probably) deadlock situation
	for _, index := range indexes {
		// We don't want the index creation to stop if for whatever reason the parent request fails or is stopped
		// in particular, if it just finishes.
		// We still put a high timeout on it to protect agains an index creation that takes probihitively long
		// An error log is sent from within createIndexSQL and should be monitored
		createIndexSQL(ctx, exec, index)
	}
}

func createIndexSQL(ctx context.Context, exec Executor, index models.ConcreteIndex) error {
	logger := utils.LoggerFromContext(ctx)
	qualifiedTableName := tableNameWithSchema(exec, index.TableName)
	indexName := indexToIndexName(index)
	indexedColumns := index.Indexed
	includedColumns := index.Included
	sql := fmt.Sprintf(
		"CREATE INDEX CONCURRENTLY %s ON %s USING btree (%s)",
		indexName,
		qualifiedTableName,
		strings.Join(pure_utils.Map(indexedColumns, withDesc), ","),
	)
	if len(includedColumns) > 0 {
		sql += fmt.Sprintf(
			" INCLUDE (%s)",
			strings.Join(
				pure_utils.Map(includedColumns, func(s models.FieldName) string { return string(s) }),
				",",
			),
		)
	}
	if _, err := exec.Exec(ctx, sql); err != nil {
		errMessage := fmt.Sprintf(
			"Error while creating index in schema %s with DDL \"%s\"",
			exec.DatabaseSchema().Schema,
			sql,
		)
		logger.ErrorContext(ctx, errMessage)
		utils.LogAndReportSentryError(ctx, err)
		return errors.Wrap(err, errMessage)
	}
	logger.InfoContext(ctx, fmt.Sprintf(
		"Index %s created in schema %s with DDL \"%s\"",
		indexName,
		exec.DatabaseSchema().Schema,
		sql,
	))
	return nil
}

func withDesc(s models.FieldName) string {
	return fmt.Sprintf("%s DESC", s)
}

func indexToIndexName(index models.ConcreteIndex) string {
	// postgresql enforces a 63 character length limit on all identifiers
	indexedNames := strings.Join(
		pure_utils.Map(
			index.Indexed,
			func(s models.FieldName) string { return string(s) },
		),
		"-",
	)
	out := fmt.Sprintf("idx_%s_%s", index.TableName, indexedNames)
	randomId := uuid.NewString()
	return pgx.Identifier.Sanitize([]string{out[:min(len(out), 53)] + "_" + randomId})
}
