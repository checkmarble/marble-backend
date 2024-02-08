package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/pg_indexes"
	"github.com/checkmarble/marble-backend/utils"
)

type IngestedDataReadRepository interface {
	GetDbField(ctx context.Context, transaction Transaction, readParams models.DbFieldReadParams) (any, error)
	ListAllObjectsFromTable(ctx context.Context, transaction Transaction, table models.Table) ([]models.ClientObject, error)
	QueryAggregatedValue(ctx context.Context, transaction Transaction, tableName models.TableName, fieldName models.FieldName, aggregator ast.Aggregator, filters []ast.Filter) (any, error)

	// Index creation
	ListAllValidIndexes(ctx context.Context, transaction Transaction) ([]models.ConcreteIndex, error)
	CreateIndexesSync(ctx context.Context, transaction Transaction, indexes []models.ConcreteIndex) (numCreating int, err error)
}

type IngestedDataReadRepositoryImpl struct{}

func (repo *IngestedDataReadRepositoryImpl) GetDbField(ctx context.Context, transaction Transaction, readParams models.DbFieldReadParams) (any, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	if len(readParams.Path) == 0 {
		return nil, fmt.Errorf("path is empty: %w", models.BadParameterError)
	}
	row, err := repo.queryDbForField(ctx, tx, readParams)
	if err != nil {
		return nil, fmt.Errorf("error while building query for DB field: %w", err)
	}

	var output any
	err = row.Scan(&output)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("no rows scanned while reading DB: %w", models.NoRowsReadError)
	} else if err != nil {
		return nil, err
	}
	return output, nil
}

func createQueryDbForField(ctx context.Context, tx Transaction, readParams models.DbFieldReadParams) (squirrel.SelectBuilder, error) {
	triggerTable, ok := readParams.DataModel.Tables[readParams.TriggerTableName]
	if !ok {
		return squirrel.SelectBuilder{}, fmt.Errorf("table %s not found in data model", readParams.TriggerTableName)
	}
	link, ok := triggerTable.LinksToSingle[readParams.Path[0]]
	if !ok {
		return squirrel.SelectBuilder{}, fmt.Errorf("no link with name %s: %w", readParams.Path[0], models.NotFoundError)
	}

	firstTableObjectId, err := getFirstTableObjectIdFromPayload(readParams.Payload, link.ChildFieldName)
	if err != nil {
		return squirrel.SelectBuilder{}, fmt.Errorf("error while getting first path table object id from payload: %w", err)
	}

	// "first table" is the first table reached starting from the trigger table and following the path
	firstTable, ok := readParams.DataModel.Tables[link.LinkedTableName]
	if !ok {
		return squirrel.SelectBuilder{}, fmt.Errorf("no table with name %s: %w", link.LinkedTableName, models.NotFoundError)
	}
	// "last table" is the last table reached starting from the trigger table and following the path, from which the field is selected
	lastTable, err := getLastTableFromPath(readParams, firstTable)
	if err != nil {
		return squirrel.SelectBuilder{}, err
	}

	firstTableName := tableNameWithSchema(tx, firstTable.Name)
	lastTableName := tableNameWithSchema(tx, lastTable.Name)

	// setup the end table we read the field from, the beginning table we join from, and relevant filters on the latter
	query := NewQueryBuilder().
		Select(fmt.Sprintf("%s.%s", lastTableName, readParams.FieldName)).
		From(firstTableName).
		Where(squirrel.Eq{fmt.Sprintf("%s.object_id", firstTableName): firstTableObjectId}).
		Where(rowIsValid(firstTableName))

	return addJoinsOnIntermediateTables(ctx, tx, query, readParams, firstTable)
}

func (repo *IngestedDataReadRepositoryImpl) queryDbForField(ctx context.Context, tx TransactionPostgres, readParams models.DbFieldReadParams) (pgx.Row, error) {
	query, err := createQueryDbForField(ctx, tx, readParams)
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}

	sql, args, err := query.ToSql()

	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}

	row := tx.exec.QueryRow(ctx, sql, args...)
	return row, nil
}

func getFirstTableObjectIdFromPayload(payload models.PayloadReader, fieldName models.FieldName) (string, error) {
	parentObjectIdItf, _ := payload.ReadFieldFromPayload(fieldName)
	if parentObjectIdItf == nil {
		return "", fmt.Errorf("%s in payload is null", fieldName) // should not happen, as per input validation
	}
	parentObjectId, ok := parentObjectIdItf.(string)
	if !ok {
		return "", fmt.Errorf("%s in payload is not a string", fieldName) // should not happen, as per input validation
	}

	return parentObjectId, nil
}

func addJoinsOnIntermediateTables(ctx context.Context, tx Transaction, query squirrel.SelectBuilder, readParams models.DbFieldReadParams, firstTable models.Table) (squirrel.SelectBuilder, error) {
	currentTable := firstTable
	// ignore the first element of the path, as it is the starting table of the query
	for _, linkName := range readParams.Path[1:] {
		link, ok := currentTable.LinksToSingle[linkName]
		if !ok {
			return squirrel.SelectBuilder{}, fmt.Errorf("no link with name %s on table %s: %w", linkName, currentTable.Name, models.NotFoundError)
		}
		nextTable, ok := readParams.DataModel.Tables[link.LinkedTableName]
		if !ok {
			return squirrel.SelectBuilder{}, fmt.Errorf("no table with name %s: %w", link.LinkedTableName, models.NotFoundError)
		}

		currentTableName := tableNameWithSchema(tx, currentTable.Name)
		nextTableName := tableNameWithSchema(tx, nextTable.Name)
		joinClause := fmt.Sprintf(
			"%s ON %s.%s = %s.%s",
			nextTableName,
			currentTableName,
			link.ChildFieldName,
			nextTableName,
			link.ParentFieldName)
		query = query.Join(joinClause).
			Where(rowIsValid(nextTableName))

		currentTable = nextTable
	}
	return query, nil
}

func rowIsValid(tableName string) squirrel.Eq {
	return squirrel.Eq{fmt.Sprintf("%s.valid_until", tableName): "Infinity"}
}

func getLastTableFromPath(params models.DbFieldReadParams, firstTable models.Table) (models.Table, error) {
	currentTable := firstTable
	// ignore the first element of the path, as it is the starting table of the query
	for _, linkName := range params.Path[1:] {
		link, ok := currentTable.LinksToSingle[linkName]
		if !ok {
			return models.Table{}, fmt.Errorf("no link with name %s: %w", linkName, models.NotFoundError)
		}
		nextTable, ok := params.DataModel.Tables[link.LinkedTableName]
		if !ok {
			return models.Table{}, fmt.Errorf("no table with name %s: %w", link.LinkedTableName, models.NotFoundError)
		}

		currentTable = nextTable
	}
	return currentTable, nil
}

func (repo *IngestedDataReadRepositoryImpl) ListAllObjectsFromTable(ctx context.Context, transaction Transaction, table models.Table) ([]models.ClientObject, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	columnNames := models.ColumnNames(table)

	objectsAsMap, err := queryWithDynamicColumnList(ctx, tx, tableNameWithSchema(tx, table.Name), columnNames)
	if err != nil {
		return nil, err
	}

	output := make([]models.ClientObject, len(objectsAsMap))
	for i, objectAsMap := range objectsAsMap {
		object := models.ClientObject{
			TableName: table.Name,
			Data:      objectAsMap,
		}
		output[i] = object
	}

	return output, nil
}

func queryWithDynamicColumnList(ctx context.Context, tx TransactionPostgres, qualifiedTableName string, columnNames []string) ([]map[string]any, error) {
	sql, args, err := NewQueryBuilder().
		Select(columnNames...).
		From(qualifiedTableName).
		Where(rowIsValid(qualifiedTableName)).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	rows, err := tx.exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("error while querying DB: %w", err)
	}
	defer rows.Close()
	output := make([]map[string]any, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("error while fetching rows: %w", err)
		}

		objectAsMap := make(map[string]any)
		for i, columnName := range columnNames {
			objectAsMap[columnName] = values[i]
		}
		output = append(output, objectAsMap)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error while iterating over rows: %w", err)
	}

	return output, nil
}

func createQueryAggregated(ctx context.Context, transaction Transaction, tableName models.TableName, fieldName models.FieldName, aggregator ast.Aggregator, filters []ast.Filter) (squirrel.SelectBuilder, error) {
	var selectExpression string
	if aggregator == ast.AGGREGATOR_COUNT_DISTINCT {
		selectExpression = fmt.Sprintf("COUNT(DISTINCT %s)", fieldName)
	} else {
		selectExpression = fmt.Sprintf("%s(%s)", aggregator, fieldName)
	}

	qualifiedTableName := tableNameWithSchema(transaction, tableName)

	query := NewQueryBuilder().
		Select(selectExpression).
		From(qualifiedTableName).
		Where(rowIsValid(qualifiedTableName))

	var err error
	for _, filter := range filters {
		query, err = addConditionForOperator(query, qualifiedTableName, filter.FieldName, filter.Operator, filter.Value)
		if err != nil {
			return squirrel.SelectBuilder{}, err
		}
	}
	return query, nil
}

func (repo *IngestedDataReadRepositoryImpl) QueryAggregatedValue(ctx context.Context, transaction Transaction, tableName models.TableName, fieldName models.FieldName, aggregator ast.Aggregator, filters []ast.Filter) (any, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	query, err := createQueryAggregated(ctx, tx, tableName, fieldName, aggregator, filters)
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	var result any
	err = tx.exec.QueryRow(ctx, sql, args...).Scan(&result)
	if err != nil {
		return nil, fmt.Errorf("error while querying DB: %w", err)
	}

	return result, nil
}

func addConditionForOperator(query squirrel.SelectBuilder, tableName string, fieldName string, operator ast.FilterOperator, value any) (squirrel.SelectBuilder, error) {
	switch operator {
	case ast.FILTER_EQUAL, ast.FILTER_IS_IN_LIST:
		return query.Where(squirrel.Eq{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_NOT_EQUAL, ast.FILTER_IS_NOT_IN_LIST:
		return query.Where(squirrel.NotEq{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_GREATER:
		return query.Where(squirrel.Gt{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_GREATER_OR_EQUAL:
		return query.Where(squirrel.GtOrEq{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_LESSER:
		return query.Where(squirrel.Lt{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	case ast.FILTER_LESSER_OR_EQUAL:
		return query.Where(squirrel.LtOrEq{fmt.Sprintf("%s.%s", tableName, fieldName): value}), nil
	default:
		return query, fmt.Errorf("unknown operator %s: %w", operator, models.BadParameterError)
	}
}

// It might be better at its place in the data model repository... but that needs to be cleaned up first as it's in a mess
// (part of the logic in Vivien's code, part in Chris's code). So for now I leave this here but it's probably not a long term solution
func (repo *IngestedDataReadRepositoryImpl) ListAllValidIndexes(ctx context.Context, transaction Transaction) ([]models.ConcreteIndex, error) {
	pgIndexes, err := repo.listAllIndexes(ctx, transaction)
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

func (repo *IngestedDataReadRepositoryImpl) listAllIndexes(ctx context.Context, transaction Transaction) ([]pg_indexes.PGIndex, error) {
	tx := adaptClientDatabaseTransaction(transaction)

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
	rows, err := tx.exec.Query(ctx, sql, tx.databaseShema.Schema)
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
	rows, err = tx.exec.Query(ctx, "SELECT index_relid FROM pg_stat_progress_create_index")
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

func (repo *IngestedDataReadRepositoryImpl) CreateIndexesSync(ctx context.Context, transaction Transaction, indexes []models.ConcreteIndex) (int, error) {
	tx := adaptClientDatabaseTransaction(transaction)

	existing, err := repo.listAllIndexes(ctx, tx)
	if err != nil {
		return 0, errors.Wrap(err, "error while listing all indexes")
	}

	var numIndexesCreated int
	for _, index := range indexes {
		if !indexAlreadyExists(index, existing) {
			err := createIndexSQL(ctx, tx, index)
			if err != nil {
				return numIndexesCreated, errors.Wrap(err, "error in CreateIndexesSync")
			}
			numIndexesCreated++
		}
	}

	return numIndexesCreated, nil
}

func indexAlreadyExists(index models.ConcreteIndex, existingIndexes []pg_indexes.PGIndex) bool {
	for _, existingIndex := range existingIndexes {
		existing := existingIndex.AdaptConcreteIndex()
		if index.Equal(existing) {
			return true
		}
	}
	return false
}

func createIndexSQL(ctx context.Context, tx TransactionPostgres, index models.ConcreteIndex) error {
	logger := utils.LoggerFromContext(ctx)
	qualifiedTableName := tableNameWithSchema(tx, index.TableName)
	indexName := indexToIndexName(index)
	indexedColumns := index.Indexed
	includedColumns := index.Included
	sql := fmt.Sprintf("CREATE INDEX %s ON %s USING btree (%s)", indexName, qualifiedTableName, strings.Join(pure_utils.Map(indexedColumns, withDesc), ","))
	if len(includedColumns) > 0 {
		sql += fmt.Sprintf(" INCLUDE (%s)", strings.Join(pure_utils.Map(includedColumns, func(s models.FieldName) string { return string(s) }), ","))
	}
	if _, err := tx.exec.Exec(ctx, sql); err != nil {
		errMessage := fmt.Sprintf("Error while creating index in schema %s with DDL \"%s\"", tx.databaseShema.Schema, sql)
		logger.Error(errMessage)
		return errors.Wrap(err, errMessage)
	}
	return nil
}

func withDesc(s models.FieldName) string {
	return fmt.Sprintf("%s DESC", s)
}

func indexToIndexName(index models.ConcreteIndex) string {
	// postgresql enforces a 63 character length limit on all identifiers
	indexedNames := strings.Join(pure_utils.Map(index.Indexed, func(s models.FieldName) string { return string(s) }), "-")
	out := fmt.Sprintf("idx_%s_%s", index.TableName, indexedNames)
	randomId := uuid.NewString()
	return pgx.Identifier.Sanitize([]string{out[:min(len(out), 53)] + "_" + randomId})
}
