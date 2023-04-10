package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (rep *PGRepository) queryDbForField(readParams app.DbFieldReadParams) (pgx.Row, error) {
	base_object_id, ok := readParams.Payload.Data["object_id"].(string)
	if !ok {
		return nil, fmt.Errorf("object_id in payload is not a string")
	}

	firstTable := readParams.DataModel.Tables[readParams.Path[0]]
	lastTable := readParams.DataModel.Tables[readParams.Path[len(readParams.Path)-1]]

	query := rep.queryBuilder.Select(fmt.Sprintf("%s.%s", lastTable.Name, readParams.FieldName)).From(firstTable.Name)

	for i := 1; i < len(readParams.Path); i++ {
		table := readParams.DataModel.Tables[readParams.Path[i-1]]
		next_table := readParams.DataModel.Tables[readParams.Path[i]]

		link, ok := table.LinksToSingle[next_table.Name]
		if !ok {
			return nil, fmt.Errorf("No link from %s to %s: %w", table.Name, next_table.Name, operators.ErrDbReadInconsistentWithDataModel)
		}
		query = query.Join(fmt.Sprintf("%s ON %s.%s = %s.%s", next_table.Name, table.Name, link.ChildFieldName, next_table.Name, link.ParentFieldName))
	}

	query = query.Where(sq.Eq{fmt.Sprintf("%s.object_id", firstTable.Name): base_object_id})
	sql, args, err := query.ToSql()
	if err != nil {
		log.Printf("Error building the query: %s\n", err)
		return nil, err
	}

	rows := rep.db.QueryRow(context.TODO(), sql, args...)
	return rows, nil
}

func scanRowReturnValue[T pgtype.Bool | pgtype.Int2 | pgtype.Float8 | pgtype.Text | pgtype.Timestamp](row pgx.Row) (T, error) {
	var returnVariable T
	err := row.Scan(&returnVariable)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return returnVariable, fmt.Errorf("No rows scanned while reading DB: %w", app.ErrNoRowsReadInDB)
		}
		return returnVariable, err
	}
	return returnVariable, nil
}

func (rep *PGRepository) GetDbField(readParams app.DbFieldReadParams) (interface{}, error) {

	row, err := rep.queryDbForField(readParams)
	if err != nil {
		return nil, err
	}

	lastTable := readParams.DataModel.Tables[readParams.Path[len(readParams.Path)-1]]
	fieldFromModel := lastTable.Fields[readParams.FieldName]

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
