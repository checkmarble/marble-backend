package mocks

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

// Generic executor mock (tx or pool)
type Executor struct {
	mock.Mock
}

func (e *Executor) DatabaseSchema() models.DatabaseSchema {
	args := e.Called()
	return args.Get(0).(models.DatabaseSchema)
}

func (e *Executor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	arguments := e.Called(ctx, sql, args)
	return arguments.Get(0).(pgconn.CommandTag), arguments.Error(1)
}

func (e *Executor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	arguments := e.Called(ctx, sql, args)
	return arguments.Get(0).(pgx.Rows), arguments.Error(1)
}

func (e *Executor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	arguments := e.Called(ctx, sql, args)
	return arguments.Get(0).(pgx.Row)
}

func (e *Executor) Begin(ctx context.Context) (repositories.Transaction, error) {
	arguments := e.Called(ctx)
	return arguments.Get(0).(repositories.Transaction), arguments.Error(1)
}

// Tx mock
type Transaction struct {
	mock.Mock
}

func (e *Transaction) DatabaseSchema() models.DatabaseSchema {
	args := e.Called()
	return args.Get(0).(models.DatabaseSchema)
}

func (e *Transaction) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	arguments := e.Called(ctx, sql, args)
	return arguments.Get(0).(pgconn.CommandTag), arguments.Error(1)
}

func (e *Transaction) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	arguments := e.Called(ctx, sql, args)
	return arguments.Get(0).(pgx.Rows), arguments.Error(1)
}

func (e *Transaction) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	arguments := e.Called(ctx, sql, args)
	return arguments.Get(0).(pgx.Row)
}

func (e *Transaction) Commit(ctx context.Context) error {
	arguments := e.Called(ctx)
	return arguments.Error(0)
}

func (e *Transaction) Rollback(ctx context.Context) error {
	arguments := e.Called(ctx)
	return arguments.Error(0)
}

func (e *Transaction) RawTx() pgx.Tx {
	arguments := e.Called()
	return arguments.Get(0).(pgx.Tx)
}

func (e *Transaction) Begin(ctx context.Context) (repositories.Transaction, error) {
	arguments := e.Called(ctx)
	return arguments.Get(0).(repositories.Transaction), arguments.Error(1)
}
