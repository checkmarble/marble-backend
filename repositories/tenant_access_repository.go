package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repo *MarbleDbRepository) SetTenantAdministrator(ctx context.Context, tx Transaction, tenantID uuid.UUID, userID string, enabled bool) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}
	// Serialize with tenant merge, other admin changes, and identity deletion.
	var id uuid.UUID
	query, args, err := NewQueryBuilder().Select("id").From("tenants").Where(squirrel.Eq{"id": tenantID, "deleted_at": nil}).Suffix("FOR UPDATE").ToSql()
	if err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.NotFoundError
		}
		return err
	}
	var lockedUser string
	query, args, err = NewQueryBuilder().Select("id").From("users").Where(squirrel.Eq{"id": userID, "tenant_id": tenantID, "deleted_at": nil}).Where(squirrel.NotEq{"role": int(models.MARBLE_ADMIN)}).Suffix("FOR UPDATE").ToSql()
	if err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, query, args...).Scan(&lockedUser); err != nil {
		return models.NotFoundError
	}
	if err := ExecBuilder(ctx, tx, NewQueryBuilder().Update("grants").Set("revoked_at", squirrel.Expr("NOW()")).Where(squirrel.Eq{"principal_type": "user", "principal_authority": "marble", "principal_id": userID, "tenant_id": tenantID, "role": "TENANT_ADMIN", "revoked_at": nil})); err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	return ExecBuilder(ctx, tx, NewQueryBuilder().Insert("grants").Columns("id", "principal_type", "principal_authority", "principal_id", "tenant_id", "role").Values(pure_utils.NewId(), "user", "marble", userID, tenantID, "TENANT_ADMIN"))
}
