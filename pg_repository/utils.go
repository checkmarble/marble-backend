package pg_repository

import (
	"context"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type PgxPoolOrTxIface interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// Return a map[string]any of column with non nil values to use with :
//   - Update().SetMap()
//   - Insert().SetMap()
//   - Where(sq.Eq())
//
// Inspired from pgx.RowToStructByName implementation
func ColumnValueMap(input any) map[string]any {
	result := make(map[string]any)

	inputElemValue := reflect.Indirect(reflect.ValueOf(input))
	inputElemType := inputElemValue.Type()

	for _, sf := range reflect.VisibleFields(inputElemType) {
		if !sf.IsExported() {
			continue
		}
		dbTag, dbTagPresent := sf.Tag.Lookup("db")
		if !dbTagPresent {
			continue
		}
		colName := strings.Split(dbTag, ",")[0]
		if dbTag == "-" {
			// Field is ignored, skip it.
			continue
		}
		colValue := inputElemValue.FieldByName(sf.Name)
		switch colValue.Kind() {
		case reflect.Struct:
			continue
		case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Slice:
			if colValue.IsNil() {
				continue
			}
			value := reflect.Indirect(colValue).Interface()
			if reflect.ValueOf(value).Kind() == reflect.Struct {
				result[colName] = ColumnValueMap(value)
			} else {
				result[colName] = value
			}
		default:
			result[colName] = colValue.Interface()
		}
	}

	return result
}
