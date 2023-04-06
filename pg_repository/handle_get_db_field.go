package pg_repository

import (
	"context"
	"fmt"
	"log"
	"marble/marble-backend/app"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (rep *PGRepository) GetDbField(path []string, fieldName string, dataModel app.DataModel, payload app.Payload) (pgtype.Bool|pgtype.Int2, error) {

	if len(path) == 0 {
		return nil, fmt.Errorf("Path is empty")
	}
	base_object_id, ok := payload.Data["object_id"].(string)
	if !ok {
		return nil, fmt.Errorf("object_id in payload is not a string")
	}

	firstTable := dataModel.Tables[path[0]]

	lastTable := dataModel.Tables[path[len(path)-1]]
	query := rep.queryBuilder.Select(fmt.Sprintf("%s.%s", lastTable.Name, fieldName)).From(firstTable.Name)

	for i := 1; i < len(path)-1; i++ {
		table := dataModel.Tables[path[i]]
		next_table := dataModel.Tables[path[i+1]]
		link, ok := table.LinksToSingle[next_table.Name]
		if !ok {
			return nil, fmt.Errorf("No link from %s to %s", table.Name, next_table.Name)
		}
		query = query.Join(fmt.Sprintf("%s ON %s.%s = %s.%s", next_table.Name, table.Name, link.ChildFieldName, next_table.Name, link.ParentFieldName))
	}

	query = query.Where(sq.Eq{fmt.Sprintf("%s.object_id", firstTable.Name): base_object_id})
	sql, args, err := query.ToSql()
	if err != nil {
		log.Printf("Error building the query: %s\n", err)
		return nil, err
	}

	fields := lastTable.Fields
	fieldFromModel, ok := fields[fieldName]

	switch fieldFromModel.DataType {
	case app.Bool:
		return scanToVariable[pgtype.Bool](sql, args, rep.db)
	case app.Int:
		return scanToVariable[pgtype.Int2](sql, args, rep.db)
	case app.Float:
		return scanToVariable[pgtype.Float8](sql, args, rep.db)
	case app.String:
		return scanToVariable[pgtype.Text](sql, args, rep.db)
	case app.Timestamp:
		return scanToVariable[pgtype.Timestamp](sql, args, rep.db)
	default:
		return nil, fmt.Errorf("Unknown data type when reading from db: %s", fieldFromModel.DataType)
	}

}

func scanToVariable[T pgtype.Bool | pgtype.Int2 | pgtype.Float8 | pgtype.Text | pgtype.Timestamp](sql string, args []interface{}, db *pgxpool.Pool) (T, error) {
	var returnVariable T
	err := db.QueryRow(context.TODO(), sql, args...).Scan(&returnVariable)
	if err != nil {
		return returnVariable, err
	}
	return returnVariable, nil
}
