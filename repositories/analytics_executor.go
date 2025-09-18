package repositories

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/Masterminds/squirrel"
)

type AnalyticsExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type DuckDbExecutor struct {
	db *sql.DB
}

func NewDuckDbExecutor(db *sql.DB) DuckDbExecutor {
	return DuckDbExecutor{
		db: db,
	}
}

func (e DuckDbExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return e.db.QueryContext(ctx, query, args...)
}

func (e DuckDbExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return e.db.QueryRowContext(ctx, query, args...)
}

func (e DuckDbExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return e.db.ExecContext(ctx, query, args...)
}

func AnalyticsScanStruct[T any](ctx context.Context, exec AnalyticsExecutor, query squirrel.SelectBuilder) ([]T, error) {
	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	return AnalyticsRawScanStruct[T](ctx, exec, sql, args...)
}

func AnalyticsRawScanStruct[T any](ctx context.Context, exec AnalyticsExecutor, sql string, args ...any) ([]T, error) {
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
