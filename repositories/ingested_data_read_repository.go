package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type IngestedDataReadRepository interface {
	GetDbField(ctx context.Context, exec Executor, readParams models.DbFieldReadParams) (any, error)
	ListAllObjectsFromTable(
		ctx context.Context,
		exec Executor,
		table models.Table,
		filters ...models.Filter,
	) ([]models.ClientObject, error)
	QueryIngestedObject(
		ctx context.Context,
		exec Executor,
		table models.Table,
		objectId string,
	) ([]map[string]any, error)
	QueryAggregatedValue(
		ctx context.Context,
		exec Executor,
		tableName string,
		fieldName string,
		fieldType models.DataType,
		aggregator ast.Aggregator,
		filters []ast.Filter,
	) (any, error)
}

type IngestedDataReadRepositoryImpl struct{}

// "read db field" methods
func (repo *IngestedDataReadRepositoryImpl) GetDbField(ctx context.Context, exec Executor, readParams models.DbFieldReadParams) (any, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	if len(readParams.Path) == 0 {
		return nil, fmt.Errorf("path is empty: %w", models.BadParameterError)
	}
	row, err := repo.queryDbForField(ctx, exec, readParams)
	if err != nil {
		return nil, fmt.Errorf("error while building query for DB field: %w", err)
	}

	var output any
	err = row.Scan(&output)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("no rows scanned while reading DB: %w", ast.ErrNoRowsRead)
	} else if err != nil {
		return nil, err
	}
	return output, nil
}

func (repo *IngestedDataReadRepositoryImpl) queryDbForField(ctx context.Context, exec Executor, readParams models.DbFieldReadParams) (pgx.Row, error) {
	query, err := createQueryDbForField(exec, readParams)
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}

	row := exec.QueryRow(ctx, sql, args...)
	return row, nil
}

func createQueryDbForField(exec Executor, readParams models.DbFieldReadParams) (squirrel.SelectBuilder, error) {
	triggerTable, ok := readParams.DataModel.Tables[readParams.TriggerTableName]
	if !ok {
		return squirrel.SelectBuilder{}, fmt.Errorf("table %s not found in data model", readParams.TriggerTableName)
	}
	link, ok := triggerTable.LinksToSingle[readParams.Path[0]]
	if !ok {
		return squirrel.SelectBuilder{}, fmt.Errorf("no link with name %s: %w",
			readParams.Path[0], models.NotFoundError)
	}

	firstTableLinkValue, err := getParentTableJoinField(readParams.ClientObject, link.ChildFieldName)
	if err != nil {
		return squirrel.SelectBuilder{}, fmt.Errorf(
			"error while getting first path table unique id from payload: %w", err)
	}

	// "first table" is the first table reached starting from the trigger table and following the path
	firstTable, ok := readParams.DataModel.Tables[link.ParentTableName]
	if !ok {
		return squirrel.SelectBuilder{}, fmt.Errorf("no table with name %s: %w",
			link.ParentTableName, models.NotFoundError)
	}
	// "last table" is the last table reached starting from the trigger table and following the path, from which the field is selected
	lastTable, err := getLastTableFromPath(readParams, firstTable)
	if err != nil {
		return squirrel.SelectBuilder{}, err
	}

	firstTableName := tableNameWithSchema(exec, firstTable.Name)
	lastTableName := tableNameWithSchema(exec, lastTable.Name)

	// setup the end table we read the field from, the beginning table we join from, and relevant filters on the latter
	query := NewQueryBuilder().
		Select(fmt.Sprintf("%s.%s", lastTableName, readParams.FieldName)).
		From(firstTableName).
		Where(squirrel.Eq{fmt.Sprintf("%s.%s", firstTableName, link.ParentFieldName): firstTableLinkValue}).
		Where(rowIsValid(firstTableName))

	return addJoinsOnIntermediateTables(exec, query, readParams, firstTable)
}

func getParentTableJoinField(payload models.ClientObject, fieldName string) (string, error) {
	parentFieldItf := payload.Data[fieldName]
	if parentFieldItf == nil {
		return "", errors.Wrap(
			ast.ErrNullFieldRead,
			fmt.Sprintf("%s in payload is null", fieldName))
	}
	parentField, ok := parentFieldItf.(string)
	if !ok {
		return "", fmt.Errorf("%s in payload is not a string", fieldName)
	}

	return parentField, nil
}

func getLastTableFromPath(params models.DbFieldReadParams, firstTable models.Table) (models.Table, error) {
	currentTable := firstTable
	// ignore the first element of the path, as it is the starting table of the query
	for _, linkName := range params.Path[1:] {
		link, ok := currentTable.LinksToSingle[linkName]
		if !ok {
			return models.Table{}, fmt.Errorf("no link with name %s: %w", linkName, models.NotFoundError)
		}
		nextTable, ok := params.DataModel.Tables[link.ParentTableName]
		if !ok {
			return models.Table{}, fmt.Errorf("no table with name %s: %w",
				link.ParentTableName, models.NotFoundError)
		}

		currentTable = nextTable
	}
	return currentTable, nil
}

func addJoinsOnIntermediateTables(
	exec Executor,
	query squirrel.SelectBuilder,
	readParams models.DbFieldReadParams,
	firstTable models.Table,
) (squirrel.SelectBuilder, error) {
	currentTable := firstTable
	// ignore the first element of the path, as it is the starting table of the query
	for _, linkName := range readParams.Path[1:] {
		link, ok := currentTable.LinksToSingle[linkName]
		if !ok {
			return squirrel.SelectBuilder{}, fmt.Errorf(
				"no link with name %s on table %s: %w", linkName, currentTable.Name, models.NotFoundError)
		}
		nextTable, ok := readParams.DataModel.Tables[link.ParentTableName]
		if !ok {
			return squirrel.SelectBuilder{}, fmt.Errorf("no table with name %s: %w",
				link.ParentTableName, models.NotFoundError)
		}

		currentTableName := tableNameWithSchema(exec, currentTable.Name)
		nextTableName := tableNameWithSchema(exec, nextTable.Name)
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

// "list all fields" methods
func (repo *IngestedDataReadRepositoryImpl) ListAllObjectsFromTable(
	ctx context.Context,
	exec Executor,
	table models.Table,
	filters ...models.Filter,
) ([]models.ClientObject, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	columnNames := models.ColumnNames(table)

	objectsAsMap, err := queryWithDynamicColumnList(
		ctx,
		exec,
		tableNameWithSchema(exec, table.Name),
		columnNames,
		filters...,
	)
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

func (repo *IngestedDataReadRepositoryImpl) QueryIngestedObject(
	ctx context.Context,
	exec Executor,
	table models.Table,
	objectId string,
) ([]map[string]any, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	columnNames := models.ColumnNames(table)

	qualifiedTableName := tableNameWithSchema(exec, table.Name)
	objectsAsMap, err := queryWithDynamicColumnList(
		ctx,
		exec,
		qualifiedTableName,
		columnNames,
		[]models.Filter{{
			LeftSql:    fmt.Sprintf("%s.object_id", qualifiedTableName),
			Operator:   ast.FUNC_EQUAL,
			RightValue: objectId,
		}}...,
	)
	if err != nil {
		return nil, err
	}

	return objectsAsMap, nil
}

func queryWithDynamicColumnList(
	ctx context.Context,
	exec Executor,
	qualifiedTableName string,
	columnNames []string,
	filters ...models.Filter,
) ([]map[string]any, error) {
	q := NewQueryBuilder().
		Select(columnNames...).
		From(qualifiedTableName).
		Where(rowIsValid(qualifiedTableName))
	for _, f := range filters {
		sql, args := f.ToSql()
		q = q.Where(sql, args...)
	}

	sql, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	rows, err := exec.Query(ctx, sql, args...)
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

func createQueryAggregated(
	exec Executor,
	tableName string,
	fieldName string,
	fieldType models.DataType,
	aggregator ast.Aggregator,
	filters []ast.Filter,
) (squirrel.SelectBuilder, error) {
	var selectExpression string
	if aggregator == ast.AGGREGATOR_COUNT_DISTINCT {
		selectExpression = fmt.Sprintf("COUNT(DISTINCT %s)", fieldName)
	} else if aggregator == ast.AGGREGATOR_COUNT {
		// COUNT(*) is a special case, as it does not take a field name (we do not want to count only non-null
		// values of a field, but all rows in the table that match the filters)
		selectExpression = "COUNT(*)"
	} else if fieldType == models.Int {
		// pgx will build a math/big.Int if we sum postgresql "bigint" (int64) values - we'd rather have a float64.
		selectExpression = fmt.Sprintf("%s(%s)::float8", aggregator, fieldName)
	} else {
		selectExpression = fmt.Sprintf("%s(%s)", aggregator, fieldName)
	}

	qualifiedTableName := tableNameWithSchema(exec, tableName)

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

func (repo *IngestedDataReadRepositoryImpl) QueryAggregatedValue(
	ctx context.Context,
	exec Executor,
	tableName string,
	fieldName string,
	fieldType models.DataType,
	aggregator ast.Aggregator,
	filters []ast.Filter,
) (any, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	query, err := createQueryAggregated(exec, tableName, fieldName, fieldType, aggregator, filters)
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	var result any
	err = exec.QueryRow(ctx, sql, args...).Scan(&result)
	if err != nil {
		return nil, fmt.Errorf("error while querying DB: %w", err)
	}

	return result, nil
}

func addConditionForOperator(query squirrel.SelectBuilder, tableName string, fieldName string,
	operator ast.FilterOperator, value any,
) (squirrel.SelectBuilder, error) {
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
