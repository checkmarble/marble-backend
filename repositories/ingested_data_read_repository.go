package repositories

import (
	"context"
	"fmt"
	"slices"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type IngestedDataReadRepository interface {
	GetDbField(ctx context.Context, exec Executor, readParams models.DbFieldReadParams) (any, error)
	ListAllObjectIdsFromTable(
		ctx context.Context,
		exec Executor,
		tableName string,
		filters ...models.Filter,
	) ([]string, error)
	QueryIngestedObject(
		ctx context.Context,
		exec Executor,
		table models.Table,
		objectId string,
	) ([]models.DataModelObject, error)
	QueryIngestedObjectByUniqueField(
		ctx context.Context,
		exec Executor,
		table models.Table,
		uniqueFieldValue string,
		uniqueFieldName string,
	) ([]models.DataModelObject, error)
	QueryAggregatedValue(
		ctx context.Context,
		exec Executor,
		tableName string,
		fieldName string,
		fieldType models.DataType,
		aggregator ast.Aggregator,
		filters []models.FilterWithType,
	) (any, error)
	ListIngestedObjects(
		ctx context.Context,
		exec Executor,
		table models.Table,
		params models.ExplorationOptions,
		cursorId *string,
		limit int,
	) ([]models.DataModelObject, error)
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
	nullFilter, query, err := createQueryDbForField(exec, readParams)
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	if nullFilter {
		return nil, nil
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error while building SQL query: %w", err)
	}
	row := exec.QueryRow(ctx, sql, args...)

	var output any
	err = row.Scan(&output)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return output, nil
}

func createQueryDbForField(exec Executor, readParams models.DbFieldReadParams) (nullFilter bool, b squirrel.SelectBuilder, err error) {
	triggerTable, ok := readParams.DataModel.Tables[readParams.TriggerTableName]
	if !ok {
		return false, b, fmt.Errorf("table %s not found in data model", readParams.TriggerTableName)
	}
	link, ok := triggerTable.LinksToSingle[readParams.Path[0]]
	if !ok {
		return false, b, fmt.Errorf("no link with name %s: %w",
			readParams.Path[0], models.NotFoundError)
	}

	// First get the value of the foreign key in the payload, following the path. If it is null, then the query should return a null value.
	parentFieldItf := readParams.ClientObject.Data[link.ChildFieldName]
	if parentFieldItf == nil {
		return true, b, nil
	}
	firstTableLinkValue, ok := parentFieldItf.(string)
	if !ok {
		return false, b, fmt.Errorf("%s in payload is not a string", link.ChildFieldName)
	}

	// "first table" is the first table reached starting from the trigger table and following the path
	firstTable, ok := readParams.DataModel.Tables[link.ParentTableName]
	if !ok {
		return false, b, fmt.Errorf("no table with name %s: %w",
			link.ParentTableName, models.NotFoundError)
	}

	// We alias all tables in the successive joins as "table_i" where i is the index of the table in the path
	// This is because a given table can appear multiple times in the path, and we need to distinguish between them
	// or the generated SQL is ambiguous.
	// NB "table_0" would be the trigger table, but it's not used in the query
	firstTableName := pgIdentifierWithSchema(exec, firstTable.Name)
	firstTableAlias := "table_1"
	// "last table" is the last table reached starting from the trigger table and following the path, from which the field is selected.
	// Exactly which table this is is detedmined below
	lastTableAlias := fmt.Sprintf("table_%d", len(readParams.Path))

	// setup the end table we read the field from, the beginning table we join from, and relevant filters on the latter
	query := NewQueryBuilder().
		Select(fmt.Sprintf("%s.%s", lastTableAlias, readParams.FieldName)).
		From(fmt.Sprintf("%s AS %s", firstTableName, firstTableAlias)).
		Where(squirrel.Eq{fmt.Sprintf("%s.%s", firstTableAlias, link.ParentFieldName): firstTableLinkValue}).
		Where(rowIsValid(firstTableAlias))

	b, err = addJoinsOnIntermediateTables(exec, query, readParams, firstTable)
	return false, b, err
}

func addJoinsOnIntermediateTables(
	exec Executor,
	query squirrel.SelectBuilder,
	readParams models.DbFieldReadParams,
	firstTable models.Table,
) (squirrel.SelectBuilder, error) {
	currentTable := firstTable
	// ignore the first element of the path, as it is the starting table of the query
	for i := 1; i < len(readParams.Path); i++ {
		linkName := readParams.Path[i]
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

		aliasCurrentTable := fmt.Sprintf("table_%d", i)
		nextTableName := pgIdentifierWithSchema(exec, nextTable.Name)
		aliastNextTable := fmt.Sprintf("table_%d", i+1)

		joinClause := fmt.Sprintf(
			"%s AS %s ON %s.%s = %s.%s",
			nextTableName,
			aliastNextTable,
			aliasCurrentTable,
			link.ChildFieldName,
			aliastNextTable,
			link.ParentFieldName)
		query = query.
			Join(joinClause).
			Where(rowIsValid(aliastNextTable))

		currentTable = nextTable
	}
	return query, nil
}

func rowIsValid(tableName string) squirrel.Eq {
	return squirrel.Eq{fmt.Sprintf("%s.valid_until", tableName): "Infinity"}
}

func (repo *IngestedDataReadRepositoryImpl) ListAllObjectIdsFromTable(
	ctx context.Context,
	exec Executor,
	tableName string,
	filters ...models.Filter,
) ([]string, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	qualifiedTableName := pgIdentifierWithSchema(exec, tableName)
	q := NewQueryBuilder().
		Select("object_id").
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

	output := make([]string, 0)
	var objectId string
	for rows.Next() {
		err = rows.Scan(&objectId)
		if err != nil {
			return nil, fmt.Errorf("error while scanning row: %w", err)
		}

		output = append(output, objectId)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error while iterating over rows: %w", err)
	}

	return output, nil
}

func (repo *IngestedDataReadRepositoryImpl) QueryIngestedObject(
	ctx context.Context,
	exec Executor,
	table models.Table,
	objectId string,
) ([]models.DataModelObject, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	columnNames := models.ColumnNames(table)

	qualifiedTableName := pgIdentifierWithSchema(exec, table.Name)
	objectsAsMap, err := queryWithDynamicColumnList(
		ctx,
		exec,
		qualifiedTableName,
		append(columnNames, "valid_from"),
		[]models.Filter{{
			LeftSql:    fmt.Sprintf("%s.object_id", qualifiedTableName),
			Operator:   ast.FUNC_EQUAL,
			RightValue: objectId,
		}}...,
	)
	if err != nil {
		return nil, err
	}

	ingestedObjects := make([]models.DataModelObject, len(objectsAsMap))
	for i, object := range objectsAsMap {
		ingestedObject := models.DataModelObject{Data: map[string]any{}, Metadata: map[string]any{}}
		for fieldName, fieldValue := range object {
			if slices.Contains(columnNames, fieldName) {
				ingestedObject.Data[fieldName] = fieldValue
			} else {
				ingestedObject.Metadata[fieldName] = fieldValue
			}
		}
		ingestedObjects[i] = ingestedObject
	}

	return ingestedObjects, nil
}

func (repo *IngestedDataReadRepositoryImpl) QueryIngestedObjectByUniqueField(
	ctx context.Context,
	exec Executor,
	table models.Table,
	uniqueFieldValue string,
	uniqueFieldName string,
) ([]models.DataModelObject, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	columnNames := models.ColumnNames(table)

	qualifiedTableName := pgIdentifierWithSchema(exec, table.Name)
	objectsAsMap, err := queryWithDynamicColumnList(
		ctx,
		exec,
		qualifiedTableName,
		append(columnNames, "valid_from"),
		[]models.Filter{{
			LeftSql:    fmt.Sprintf("%s.%s", qualifiedTableName, uniqueFieldName),
			Operator:   ast.FUNC_EQUAL,
			RightValue: uniqueFieldValue,
		}}...,
	)
	if err != nil {
		return nil, err
	}

	ingestedObjects := make([]models.DataModelObject, len(objectsAsMap))
	for i, object := range objectsAsMap {
		ingestedObject := models.DataModelObject{Data: map[string]any{}, Metadata: map[string]any{}}
		for fieldName, fieldValue := range object {
			if slices.Contains(columnNames, fieldName) {
				ingestedObject.Data[fieldName] = fieldValue
			} else {
				ingestedObject.Metadata[fieldName] = fieldValue
			}
		}
		ingestedObjects[i] = ingestedObject
	}

	return ingestedObjects, nil
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
	filters []models.FilterWithType,
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

	qualifiedTableName := pgIdentifierWithSchema(exec, tableName)

	query := NewQueryBuilder().
		Select(selectExpression).
		From(qualifiedTableName).
		Where(rowIsValid(qualifiedTableName))

	var err error
	for _, filter := range filters {
		query, err = addConditionForOperator(query, qualifiedTableName,
			filter.Filter.FieldName, filter.FieldType, filter.Filter.Operator, filter.Filter.Value)
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
	filters []models.FilterWithType,
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

func addConditionForOperator(query squirrel.SelectBuilder, tableName string, fieldName string, fieldType models.DataType,
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
	case ast.FILTER_IS_EMPTY:
		orCondition := squirrel.Or{
			squirrel.Eq{fmt.Sprintf("%s.%s", tableName, fieldName): nil},
		}
		if fieldType == models.String {
			orCondition = append(orCondition, squirrel.Eq{
				fmt.Sprintf("%s.%s", tableName, fieldName): "",
			})
		}
		return query.Where(orCondition), nil
	case ast.FILTER_IS_NOT_EMPTY:
		andCondition := squirrel.And{
			squirrel.NotEq{fmt.Sprintf("%s.%s", tableName, fieldName): nil},
		}
		if fieldType == models.String {
			andCondition = append(andCondition,
				squirrel.NotEq{fmt.Sprintf("%s.%s", tableName, fieldName): ""},
			)
		}
		return query.Where(andCondition), nil
	case ast.FILTER_STARTS_WITH:
		return query.Where(squirrel.Like{fmt.Sprintf("%s.%s", tableName, fieldName): fmt.Sprintf("%s%%", value)}), nil
	case ast.FILTER_ENDS_WITH:
		return query.Where(squirrel.Like{fmt.Sprintf("%s.%s", tableName, fieldName): fmt.Sprintf("%%%s", value)}), nil
	case ast.FILTER_FUZZY_MATCH:
		return query.Where(fmt.Sprintf("similarity(%s.%s, ?) > 0.6", tableName, fieldName), value), nil
	default:
		return query, fmt.Errorf("unknown operator %s: %w", operator, models.BadParameterError)
	}
}

func (repo *IngestedDataReadRepositoryImpl) ListIngestedObjects(
	ctx context.Context,
	exec Executor,
	table models.Table,
	params models.ExplorationOptions,
	cursorId *string,
	limit int,
) ([]models.DataModelObject, error) {
	if err := validateClientDbExecutor(exec); err != nil {
		return nil, err
	}

	tableColumnNames := models.ColumnNames(table)
	columnNames := append(tableColumnNames, "valid_from")
	qualifiedTableName := pgIdentifierWithSchema(exec, table.Name)
	filterFieldName := pgIdentifierWithSchema(exec, table.Name,
		params.FilterFieldName)
	qualifiedOrderingField := pgIdentifierWithSchema(exec, table.Name,
		params.OrderingFieldName)

	type pagination struct {
		fields string
		values []any
	}
	var paginationValues *pagination
	if cursorId != nil {
		cursorObjects, err := repo.QueryIngestedObject(ctx, exec, table, *cursorId)
		if err != nil {
			return nil, errors.Wrap(err, "error while querying DB for cursor in ListIngestedObjects")
		}
		if len(cursorObjects) == 0 {
			return nil, errors.Wrap(models.NotFoundError, "cursor not found")
		}
		cursorObject := cursorObjects[0]
		orderFieldVal, ok := cursorObject.Data[params.OrderingFieldName]
		if !ok {
			return nil, errors.Newf("field %s not found in cursor object",
				params.OrderingFieldName)
		}
		cursorObjectId, ok := cursorObject.Data["object_id"]
		if !ok {
			return nil, errors.Newf("field %s not found in cursor object", "object_id")
		}
		paginationValues = &pagination{
			fields: fmt.Sprintf("(%s, %s) < (?, ?)", qualifiedOrderingField, "object_id"),
			values: []any{orderFieldVal, cursorObjectId},
		}

	}

	var filterFieldValue any
	if params.FilterFieldValue.StringValue != nil {
		filterFieldValue = *params.FilterFieldValue.StringValue
	} else if params.FilterFieldValue.FloatValue != nil {
		filterFieldValue = *params.FilterFieldValue.FloatValue
	} else {
		return nil, errors.New("invalid nil filter field value")
	}

	q := NewQueryBuilder().
		Select(columnNames...).
		From(qualifiedTableName).
		Where(squirrel.Eq{
			filterFieldName: filterFieldValue,
			fmt.Sprintf("%s.valid_until", qualifiedTableName): "Infinity",
		}).
		OrderBy(qualifiedOrderingField+" DESC", "object_id DESC").
		Limit(uint64(limit))
	if paginationValues != nil {
		q = q.Where(paginationValues.fields, paginationValues.values...)
	}

	sql, args, err := q.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error while building SQL query in ListIngestedObjects")
	}

	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error while querying DB in ListIngestedObjects")
	}

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.DataModelObject, error) {
		values, err := row.Values()
		if err != nil {
			return models.DataModelObject{}, errors.Wrap(err,
				"error while fetching rows in ListIngestedObjects")
		}

		ingestedObject := models.DataModelObject{Data: map[string]any{}, Metadata: map[string]any{}}
		for i, columnName := range columnNames {
			if slices.Contains(tableColumnNames, columnName) {
				ingestedObject.Data[columnName] = values[i]
			} else {
				ingestedObject.Metadata[columnName] = values[i]
			}
		}

		return ingestedObject, nil
	})
}
