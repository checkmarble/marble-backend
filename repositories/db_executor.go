package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type transactionOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// implements the DbExecutor from usecasess
type ExecutorPostgres struct {
	databaseShema models.DatabaseSchema
	exec          transactionOrPool
}

func (e ExecutorPostgres) DatabaseSchema() models.DatabaseSchema {
	return e.databaseShema
}

func (e ExecutorPostgres) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return e.exec.Exec(ctx, sql, args...)
}

func (e ExecutorPostgres) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return e.exec.Query(ctx, sql, args...)
}

func (e ExecutorPostgres) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return e.exec.QueryRow(ctx, sql, args...)
}

type SqlBuilder interface {
	ToSql() (string, []interface{}, error)
}

func ExecBuilder(ctx context.Context, exec Executor, builder SqlBuilder) (rowsAffected int64, err error) {
	query, args, err := builder.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "can't build sql query")
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("error executing sql query: %s", query))
	}
	return tag.RowsAffected(), nil
}
