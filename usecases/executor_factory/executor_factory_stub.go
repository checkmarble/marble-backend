package executor_factory

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
)

type ExecutorFactoryStub struct {
	Mock pgxmock.PgxPoolIface
}

func NewExecutorFactoryStub() ExecutorFactoryStub {
	pool, _ := pgxmock.NewPool()

	return ExecutorFactoryStub{
		Mock: pool,
	}
}

type PgExecutorStub struct {
	pgxmock.PgxPoolIface
}

func (stub ExecutorFactoryStub) NewClientDbExecutor(ctx context.Context, organizationId string) (repositories.Executor, error) {
	return nil, nil
}

func (stub ExecutorFactoryStub) NewExecutor() repositories.Executor {
	return PgExecutorStub{
		stub.Mock,
	}
}

func (stub PgExecutorStub) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{}
}

// TransactionFactoryStub is a stub for the transaction factory

type TransactionFactoryStub struct {
	Mock dbExecFactoryStub
}

func NewTransactionFactoryStub(exec ExecutorFactoryStub) TransactionFactoryStub {
	return TransactionFactoryStub{
		Mock: NewDbExecFactoryStub(exec.Mock),
	}
}

func (stub TransactionFactoryStub) Transaction(ctx context.Context, fn func(tx repositories.Transaction) error) error {
	err := fn(stub.Mock)
	return err
}

func (stub TransactionFactoryStub) TransactionInOrgSchema(
	ctx context.Context,
	organizationId string,
	f func(tx repositories.Transaction) error,
) error {
	exec := stub.Mock.withClientSchema()
	err := f(exec)
	return err
}

// helper type to inject to the tx factory stub

type dbExecFactoryStub struct {
	exec       pgxmock.PgxPoolIface
	schemaType models.DatabaseSchemaType
}

func NewDbExecFactoryStub(exec pgxmock.PgxPoolIface) dbExecFactoryStub {
	return dbExecFactoryStub{
		exec: exec,
	}
}

func (exec dbExecFactoryStub) withClientSchema() dbExecFactoryStub {
	exec.schemaType = models.DATABASE_SCHEMA_TYPE_CLIENT
	return exec
}

func (exec dbExecFactoryStub) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{
		SchemaType: exec.schemaType,
		Schema:     "test",
	}
}

func (stub dbExecFactoryStub) RawTx() pgx.Tx {
	return stub.exec
}

func (stub dbExecFactoryStub) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	return stub.exec.Exec(ctx, sql, arguments...)
}

func (stub dbExecFactoryStub) Query(ctx context.Context, sql string, arguments ...interface{}) (pgx.Rows, error) {
	return stub.exec.Query(ctx, sql, arguments...)
}

func (stub dbExecFactoryStub) QueryRow(ctx context.Context, sql string, arguments ...interface{}) pgx.Row {
	return stub.exec.QueryRow(ctx, sql, arguments...)
}
