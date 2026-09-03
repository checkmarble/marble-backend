package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

type GrantRepository interface {
	EnsureTenantAdminForOrganization(ctx context.Context, exec Executor, userID string, organizationID uuid.UUID) error
}

func (repo *MarbleDbRepository) EnsureTenantAdminForOrganization(ctx context.Context, exec Executor, userID string, organizationID uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	selectQuery := NewQueryBuilder().Select().
		Column("?", pure_utils.NewId()).
		Column("?", "user").
		Column("?", userID).
		Column("?", "marble").
		Column("tenant_id").
		Column("?", models.TENANT_ADMIN.String()).
		From(dbmodels.TABLE_ORGANIZATION).
		Where(squirrel.Eq{"id": organizationID})

	return ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_GRANTS).
			Columns(
				"id",
				"principal_type",
				"principal_id",
				"principal_authority",
				"tenant_id",
				"role",
			).
			Select(selectQuery).
			Suffix("ON CONFLICT DO NOTHING"),
	)
}
