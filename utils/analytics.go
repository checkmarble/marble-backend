package utils

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/Masterminds/squirrel"
)

func ScanStruct[T any](ctx context.Context, exec *sql.DB, query squirrel.SelectBuilder) ([]T, error) {
	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	tmp := *new(T)

	rt := reflect.TypeOf(tmp)
	rv := reflect.ValueOf(&tmp).Elem()
	ptrs := make([]any, rt.NumField())

	for idx := range rt.NumField() {
		ptrs[idx] = rv.Field(idx).Addr().Interface()
	}

	rows, err := exec.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	results := make([]T, 0)

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		results = append(results, tmp)
	}

	return results, rows.Err()
}
