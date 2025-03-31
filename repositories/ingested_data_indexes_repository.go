package repositories

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/pg_indexes"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

var INDEX_CREATION_TIMEOUT time.Duration = 60 * 4 * time.Minute // 4 hours

func (repo *ClientDbRepository) ListAllValidIndexes(
	ctx context.Context,
	exec Executor,
	indexTypes ...models.IndexType,
) ([]models.ConcreteIndex, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	pgIndexes, err := repo.listAllPgIndexes(ctx, exec)
	if err != nil {
		return nil, errors.Wrap(err, "error while listing all indexes")
	}

	var validIndexes []models.ConcreteIndex
	for _, pgIndex := range pgIndexes {
		if len(indexTypes) > 0 && !slices.Contains(indexTypes, pgIndex.AdaptConcreteIndex().Type) {
			continue
		}
		if pgIndex.IsValid {
			validIndexes = append(validIndexes, pgIndex.AdaptConcreteIndex())
		}
	}

	return validIndexes, nil
}

func (repo *ClientDbRepository) ListAllIndexes(
	ctx context.Context,
	exec Executor,
	indexTypes ...models.IndexType,
) ([]models.ConcreteIndex, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	pgIndexes, err := repo.listAllPgIndexes(ctx, exec)
	if err != nil {
		return nil, errors.Wrap(err, "error while listing all indexes")
	}

	indexes := make([]models.ConcreteIndex, 0, len(pgIndexes))
	for _, pgIndex := range pgIndexes {
		if len(indexTypes) > 0 && !slices.Contains(indexTypes, pgIndex.AdaptConcreteIndex().Type) {
			continue
		}
		indexes = append(indexes, pgIndex.AdaptConcreteIndex())
	}

	return indexes, nil
}

func (repo *ClientDbRepository) ListAllUniqueIndexes(
	ctx context.Context,
	exec Executor,
) ([]models.UnicityIndex, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	pgIndexes, err := repo.listAllPgIndexes(ctx, exec)
	if err != nil {
		return nil, errors.Wrap(err, "error while listing all indexes")
	}

	var uniqueIndexes []models.UnicityIndex
	for _, pgIndex := range pgIndexes {
		if pgIndex.IsUnique {
			isUnique, idx := pgIndex.AdaptUnicityIndex()
			if isUnique {
				uniqueIndexes = append(uniqueIndexes, idx)
			}
		}
	}

	return uniqueIndexes, nil
}

func (repo *ClientDbRepository) CountPendingIndexes(
	ctx context.Context,
	exec Executor,
) (int, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return 0, err
	}

	pgIndexes, err := repo.listAllPgIndexes(ctx, exec)
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

func (repo *ClientDbRepository) listAllPgIndexes(
	ctx context.Context,
	exec Executor,
) ([]pg_indexes.PGIndex, error) {
	sql := `
	SELECT
		pg_get_indexdef(pg_class_idx.oid) AS indexdef,
		pg_class_idx.relname AS indexname,
		pgidx.indisvalid,
		pgidx.indexrelid,
		pgidx.indisunique,
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
		err := row.Scan(&index.Definition, &index.Name, &index.IsValid, &index.RelationId, &index.IsUnique, &index.TableName)
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

func (repo *ClientDbRepository) CreateIndexesWithCallback(
	ctx context.Context,
	exec Executor,
	indexes []models.ConcreteIndex,
	onSuccess models.OnCreateIndexesSuccess,
) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	go func() {
		ctx = context.WithoutCancel(ctx)
		ctx, cancel := context.WithTimeout(ctx, INDEX_CREATION_TIMEOUT)
		defer cancel()
		for _, index := range indexes {
			err := createIndexSQL(ctx, exec, index)
			if err != nil {
				utils.LogAndReportSentryError(ctx, err)
				return
			}
		}

		if onSuccess != nil {
			err := onSuccess(ctx)
			if err != nil {
				utils.LogAndReportSentryError(ctx, err)
			}
		}
	}()
	return nil
}

func asynchronouslyCreateIndexes(
	ctx context.Context,
	exec Executor,
	indexes []models.ConcreteIndex,
) {
	ctx = context.WithoutCancel(ctx)
	ctx, _ = context.WithTimeout(ctx, INDEX_CREATION_TIMEOUT) //nolint:govet
	// The function is meant to be executed asynchronously and return way after the request was finished,
	// so we don't return any error
	// However the indexes are created one after the other to avoid a (probably) deadlock situation
	for _, index := range indexes {
		// We don't want the index creation to stop if for whatever reason the parent request fails or is stopped
		// in particular, if it just finishes.
		// We still put a high timeout on it to protect agains an index creation that takes probihitively long
		// An error log is sent from within createIndexSQL and should be monitored
		_ = createIndexSQL(ctx, exec, index)
	}
}

func createIndexSQL(ctx context.Context, exec Executor, index models.ConcreteIndex) error {
	logger := utils.LoggerFromContext(ctx)
	qualifiedTableName := tableNameWithSchema(exec, index.TableName)
	indexedColumns := index.Indexed
	includedColumns := index.Included

	sql := fmt.Sprintf(
		"CREATE INDEX CONCURRENTLY %s ON %s USING btree (%s)",
		pgx.Identifier.Sanitize([]string{index.Name()}),
		qualifiedTableName,
		strings.Join(pure_utils.Map(indexedColumns, withDesc), ","),
	)
	if len(includedColumns) > 0 {
		sql += fmt.Sprintf(
			" INCLUDE (%s)",
			strings.Join(includedColumns, ","),
		)
	}
	sql += "WHERE valid_until='infinity'"

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
		index.Name(),
		exec.DatabaseSchema().Schema,
		sql,
	))
	return nil
}

func withDesc(s string) string {
	return fmt.Sprintf("%s DESC", s)
}

func toUniqIndexName(fields []string, table string) string {
	// postgresql enforces a 63 character length limit on all identifiers
	indexedNames := strings.Join(fields, "-")
	out := fmt.Sprintf("uniq_idx_%s_%s", table, indexedNames)
	length := min(len(out), 53)
	return out[:length]
}

func (repo *ClientDbRepository) CreateUniqueIndexAsync(ctx context.Context, exec Executor, index models.UnicityIndex) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	// The usecase is responsible for ensuring that a valid unique index does not exist yet. This is only for
	// cleaning up an invalid index (created concurrently) and creating a new one.
	indexName := toUniqIndexName(index.Fields, index.TableName)
	if _, err := exec.Exec(ctx, dropIdxSqlQuery(indexName, exec)); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error while dropping index %s", indexName))
	}

	ctx = context.WithoutCancel(ctx)
	ctx, _ = context.WithTimeout(ctx, INDEX_CREATION_TIMEOUT) //nolint:govet

	go createUniqueIndex(ctx, exec, index, true) //nolint:errcheck
	// The function is meant to be executed asynchronously and return way after the request was finished,
	// so we don't return any error
	return nil
}

func dropIdxSqlQuery(indexName string, exec Executor) string {
	return fmt.Sprintf(
		"DROP INDEX IF EXISTS %s",
		pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, indexName}),
	)
}

func createUniqueIndex(ctx context.Context, exec Executor, index models.UnicityIndex, async bool) error {
	logger := utils.LoggerFromContext(ctx)
	qualifiedTableName := tableNameWithSchema(exec, index.TableName)
	indexName := pgx.Identifier.Sanitize([]string{
		toUniqIndexName(index.Fields, index.TableName),
	})

	var concurrently string
	if async {
		concurrently = "CONCURRENTLY"
	}
	sql := fmt.Sprintf(
		"CREATE UNIQUE INDEX %s IF NOT EXISTS %s ON %s (%s)",
		concurrently,
		indexName,
		qualifiedTableName,
		strings.Join(pure_utils.Map(index.Fields, withDesc), ","),
	)
	if len(index.Included) > 0 {
		sql += fmt.Sprintf(
			" INCLUDE (%s)",
			strings.Join(index.Included, ","),
		)
	}
	sql += " WHERE valid_until='infinity'"

	if _, err := exec.Exec(ctx, sql); err != nil {
		errMessage := fmt.Sprintf(
			"Error while creating unique index in schema %s with DDL \"%s\"",
			exec.DatabaseSchema().Schema,
			sql,
		)
		logger.ErrorContext(ctx, errMessage)
		utils.LogAndReportSentryError(ctx, err)
		return errors.Wrap(err, errMessage)
	}
	logger.InfoContext(ctx, fmt.Sprintf(
		"Unique index %s created in schema %s with DDL \"%s\"",
		indexName,
		exec.DatabaseSchema().Schema,
		sql,
	))
	return nil
}

func (repo *ClientDbRepository) CreateUniqueIndex(ctx context.Context, exec Executor, index models.UnicityIndex) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	indexName := toUniqIndexName(index.Fields, index.TableName)
	if _, err := exec.Exec(ctx, dropIdxSqlQuery(indexName, exec)); err != nil {
		return errors.Wrap(err, "error while dropping index")
	}
	return createUniqueIndex(ctx, exec, index, false)
}

func (repo *ClientDbRepository) DeleteUniqueIndex(ctx context.Context, exec Executor, index models.UnicityIndex) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	indexName := toUniqIndexName(index.Fields, index.TableName)
	_, err := exec.Exec(ctx, dropIdxSqlQuery(indexName, exec))
	return err
}

func (repo *ClientDbRepository) ListIndicesPendingCreation(ctx context.Context, exec Executor) ([]string, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := `
		select
			coalesce(
					pgai.indexrelname,
					split_part(trim(split_part(query, 'concurrently', 2)), ' ', 1)
			) as index,
			pgi.phase,
			case
			when pgi.blocks_total > 0 then round(pgi.blocks_done / pgi.blocks_total::numeric * 100, 2)
			else 100
			end AS "% done"
		from pg_stat_activity pga
		left join pg_stat_progress_create_index pgi on pga.pid = pgi.pid
		left join pg_class pgc on pgc.oid = pgi.relid
		left join pg_stat_all_indexes pgai on pgai.relname = pgc.relname and pgai.indexrelid = pgi.index_relid
		left join pg_namespace pgn on pgn.oid  = pgc.relnamespace
		where
			pgn.nspname = $1 and
			(pga.query ilike 'create index concurrently %' or pga.query ilike 'create unique index concurrently %') and
			pga.leader_pid is null;
	`

	rows, err := exec.Query(ctx, sql, exec.DatabaseSchema().Schema)
	if err != nil {
		return nil, errors.Wrap(err, "error while querying DB to read indexes")
	}

	indices, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var indexName string

		err := row.Scan(&indexName, nil, nil)

		return indexName, err
	})

	return indices, err
}

func (editor *ClientDbRepository) ListInvalidIndices(ctx context.Context, exec Executor) ([]string, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := `
		select relname
		from pg_index pgi
		left join pg_class pgc on pgc.oid = pgi.indexrelid
		left join pg_namespace pgn on pgn.oid = pgc.relnamespace
		where
			pgn.nspname = $1 and
			pgi.indisvalid = false;
	`

	rows, err := exec.Query(ctx, sql, exec.DatabaseSchema().Schema)
	if err != nil {
		return nil, errors.Wrap(err, "error while querying DB to read indexes")
	}

	indices, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var indexName string

		err := row.Scan(&indexName)

		return indexName, err
	})

	return indices, err
}

func (editor *ClientDbRepository) DeleteInvalidIndex(ctx context.Context, exec Executor, indexName string) error {
	if err := validateClientDbExecutor(exec); err != nil {
		return err
	}

	sql := fmt.Sprintf(`drop index concurrently %s`,
		pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, indexName}))

	_, err := exec.Exec(ctx, sql)
	if err != nil {
		return errors.Wrap(err, "error while deleting invalid index")
	}

	return nil
}
