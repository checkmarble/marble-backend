package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// //////////////////////////////////
// Generic db executor (tx or pool)
// //////////////////////////////////

type pgxTxOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type TransactionOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (Transaction, error)
}

type PgExecutor struct {
	databaseSchema models.DatabaseSchema
	exec           pgxTxOrPool
}

func (e PgExecutor) DatabaseSchema() models.DatabaseSchema {
	return e.databaseSchema
}

func (e PgExecutor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tag, err := injectDbSessionConfig(ctx, e.exec, sql); err != nil {
		return tag, err
	}

	orgId := uuid.Nil
	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		orgId = creds.OrganizationId
	}

	return utils.MeasureLatencyErr(utils.MetricQueryLatency, prometheus.Labels{
		"org_id": orgId.String(), "schema": e.databaseSchema.Schema,
	}, func() (pgconn.CommandTag, error) {
		return e.exec.Exec(ctx, sql, args...)
	})
}

func (e PgExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if _, err := injectDbSessionConfig(ctx, e.exec, sql); err != nil {
		return nil, err
	}

	orgId := uuid.Nil
	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		orgId = creds.OrganizationId
	}
	return utils.MeasureLatencyErr(utils.MetricQueryLatency, prometheus.Labels{
		"org_id": orgId.String(), "schema": e.databaseSchema.Schema,
	}, func() (pgx.Rows, error) {
		return e.exec.Query(ctx, sql, args...)
	})
}

func (e PgExecutor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if _, err := injectDbSessionConfig(ctx, e.exec, sql); err != nil {
		return errorRow{err}
	}

	orgId := uuid.Nil
	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		orgId = creds.OrganizationId
	}

	return utils.MeasureLatency(utils.MetricQueryLatency, prometheus.Labels{
		"org_id": orgId.String(), "schema": e.databaseSchema.Schema,
	}, func() pgx.Row {
		return e.exec.QueryRow(ctx, sql, args...)
	})
}

func (e PgExecutor) Begin(ctx context.Context) (Transaction, error) {
	tx, err := e.exec.Begin(ctx)
	if err != nil {
		return PgTx{}, errors.Wrap(err, "Error beginning transaction")
	}
	return PgTx{
		databaseSchema: e.databaseSchema,
		tx:             tx,
	}, nil
}

////////////////////////////////////
// Transaction
////////////////////////////////////

type PgTx struct {
	databaseSchema models.DatabaseSchema
	tx             pgx.Tx
}

func (t PgTx) DatabaseSchema() models.DatabaseSchema {
	return t.databaseSchema
}

func (t PgTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	orgId := uuid.Nil
	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		orgId = creds.OrganizationId
	}

	return utils.MeasureLatencyErr(utils.MetricQueryLatency, prometheus.Labels{
		"org_id": orgId.String(), "schema": t.databaseSchema.Schema,
	}, func() (pgconn.CommandTag, error) {
		return t.tx.Exec(ctx, sql, args...)
	})
}

func (t PgTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	orgId := uuid.Nil
	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		orgId = creds.OrganizationId
	}

	return utils.MeasureLatencyErr(utils.MetricQueryLatency, prometheus.Labels{
		"org_id": orgId.String(), "schema": t.databaseSchema.Schema,
	}, func() (pgx.Rows, error) {
		return t.tx.Query(ctx, sql, args...)
	})
}

func (t PgTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	orgId := uuid.Nil
	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		orgId = creds.OrganizationId
	}

	return utils.MeasureLatency(utils.MetricQueryLatency, prometheus.Labels{
		"org_id": orgId.String(), "schema": t.databaseSchema.Schema,
	}, func() pgx.Row {
		return t.tx.QueryRow(ctx, sql, args...)
	})
}

func (t PgTx) RawTx() pgx.Tx {
	return t.tx
}

func (t PgTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t PgTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

func (t PgTx) Begin(ctx context.Context) (Transaction, error) {
	tx, err := t.tx.Begin(ctx)
	if err != nil {
		return PgTx{}, errors.Wrap(err, "Error beginning transaction")
	}
	return PgTx{
		databaseSchema: t.databaseSchema,
		tx:             tx,
	}, nil
}
