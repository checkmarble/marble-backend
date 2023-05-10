package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func rowIsValid(tableName app.TableName) sq.Eq {
	return sq.Eq{fmt.Sprintf("%s.valid_until", tableName): "Infinity"}
}

func getLastTableFromPath(path []string, dataModel app.DataModel) (app.Table, error) {
	firstTable := dataModel.Tables[app.TableName(path[0])]
	if len(path) == 1 {
		return firstTable, nil
	}

	currentTable := firstTable
	for i := 1; i < len(path); i++ {
		link, ok := currentTable.LinksToSingle[app.LinkName(path[i])]
		if !ok {
			return app.Table{}, fmt.Errorf("No link with name %s: %w", path[i], operators.ErrDbReadInconsistentWithDataModel)
		}
		nextTable := dataModel.Tables[app.TableName(link.LinkedTableName)]

		currentTable = nextTable
	}
	return currentTable, nil
}

func (rep *PGRepository) queryDbForField(ctx context.Context, readParams app.DbFieldReadParams) (pgx.Row, error) {
	baseObjectIdAny := readParams.Payload.ReadFieldFromDynamicStruct("object_id")
	baseObjectIdPtr, ok := baseObjectIdAny.(*string)
	if !ok {
		return nil, fmt.Errorf("object_id in payload is not a string") // should not happen, as per input validation
	}

	if baseObjectIdPtr == nil {
		return nil, fmt.Errorf("object_id in payload is null") // should not happen, as per input validation
	}
	baseObjectId := *baseObjectIdPtr

	firstTable := readParams.DataModel.Tables[app.TableName(readParams.Path[0])]
	lastTable, err := getLastTableFromPath(readParams.Path, readParams.DataModel)
	if err != nil {
		return nil, err
	}

	query := rep.queryBuilder.Select(fmt.Sprintf("%s.%s", lastTable.Name, readParams.FieldName)).From(string(firstTable.Name))

	currentTable := firstTable
	for i := 1; i < len(readParams.Path); i++ {
		link, ok := currentTable.LinksToSingle[app.LinkName(readParams.Path[i])]
		if !ok {
			return nil, fmt.Errorf("No link with name %s: %w", readParams.Path[i], operators.ErrDbReadInconsistentWithDataModel)
		}
		nextTable := readParams.DataModel.Tables[app.TableName(link.LinkedTableName)]

		joinClause := fmt.Sprintf("%s ON %s.%s = %s.%s", nextTable.Name, currentTable.Name, link.ChildFieldName, nextTable.Name, link.ParentFieldName)
		query = query.Join(joinClause).
			Where(rowIsValid(nextTable.Name))

		currentTable = nextTable
	}

	query = query.Where(sq.Eq{fmt.Sprintf("%s.object_id", firstTable.Name): baseObjectId}).
		Where(rowIsValid(firstTable.Name))
	sql, args, err := query.ToSql()

	if err != nil {
		return nil, fmt.Errorf("Error while building SQL query: %w", err)
	}

	rows := rep.db.QueryRow(ctx, sql, args...)
	return rows, nil
}

func scanRowReturnValue[T pgtype.Bool | pgtype.Int2 | pgtype.Float8 | pgtype.Text | pgtype.Timestamp](row pgx.Row) (T, error) {
	var returnVariable T
	err := row.Scan(&returnVariable)
	if errors.Is(err, pgx.ErrNoRows) {
		return returnVariable, fmt.Errorf("No rows scanned while reading DB: %w", app.ErrNoRowsReadInDB)
	} else if err != nil {
		return returnVariable, err
	}
	return returnVariable, nil
}

func (rep *PGRepository) GetDbField(ctx context.Context, readParams app.DbFieldReadParams) (interface{}, error) {
	row, err := rep.queryDbForField(ctx, readParams)
	if err != nil {
		return nil, fmt.Errorf("Error while building query for DB field: %w", err)
	}

	lastTable := readParams.DataModel.Tables[app.TableName(readParams.Path[len(readParams.Path)-1])]
	fieldFromModel := lastTable.Fields[app.FieldName(readParams.FieldName)]

	switch fieldFromModel.DataType {
	case app.Bool:
		return scanRowReturnValue[pgtype.Bool](row)
	case app.Int:
		return scanRowReturnValue[pgtype.Int2](row)
	case app.Float:
		return scanRowReturnValue[pgtype.Float8](row)
	case app.String:
		return scanRowReturnValue[pgtype.Text](row)
	case app.Timestamp:
		return scanRowReturnValue[pgtype.Timestamp](row)
	default:
		return nil, fmt.Errorf("Unknown data type when reading from db: %s", fieldFromModel.DataType)
	}
}
