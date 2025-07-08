package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// //////////////////////////////////
// Generic db executor (tx or pool)
// //////////////////////////////////
type TransactionOrPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type PgExecutor struct {
	databaseSchema models.DatabaseSchema
	exec           TransactionOrPool
}

func (e PgExecutor) DatabaseSchema() models.DatabaseSchema {
	return e.databaseSchema
}

func (e PgExecutor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tag, err := InjectDbSessionConfig(ctx, e.exec); err != nil {
		return tag, err
	}

	return e.exec.Exec(ctx, sql, args...)
}

func (e PgExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if _, err := InjectDbSessionConfig(ctx, e.exec); err != nil {
		return nil, err
	}

	return e.exec.Query(ctx, sql, args...)
}

func (e PgExecutor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if _, err := InjectDbSessionConfig(ctx, e.exec); err != nil {
		return nil
	}

	return e.exec.QueryRow(ctx, sql, args...)
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
	return t.tx.Exec(ctx, sql, args...)
}

func (t PgTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.tx.Query(ctx, sql, args...)
}

func (t PgTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, args...)
}

func (t PgTx) RawTx() pgx.Tx {
	return t.tx
}

func InjectDbSessionConfig(ctx context.Context, exec TransactionOrPool) (pgconn.CommandTag, error) {
	if creds, ok := utils.CredentialsFromCtx(ctx); ok && creds.ActorIdentity.UserId != "" {
		if tag, err := exec.Exec(ctx, "SELECT SET_CONFIG('custom.current_org_id', $1, false)", creds.OrganizationId); err != nil {
			return tag, err
		}
		if tag, err := exec.Exec(ctx, "SELECT SET_CONFIG('custom.current_user_id', $1, false)", creds.ActorIdentity.UserId); err != nil {
			return tag, err
		}
	}

	return pgconn.NewCommandTag(""), nil
}
