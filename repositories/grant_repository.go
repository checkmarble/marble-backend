package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type GrantRepository interface {
	EnsureTenantAdminForOrganization(ctx context.Context, exec Executor, userID string, organizationID uuid.UUID) error
	ListOrganizationsForUser(ctx context.Context, exec Executor, userID string) ([]models.OrganizationMembership, error)
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

func (repo *MarbleDbRepository) ListOrganizationsForUser(ctx context.Context, exec Executor, userID string) ([]models.OrganizationMembership, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("o.id", "o.name", "o.tenant_id", "o.environment").
		Column("array_agg(DISTINCT scope_grants.role)").
		From("active_grants membership").
		Join("organizations o ON o.id = membership.organization_id").
		Join("active_grants scope_grants ON scope_grants.principal_type = membership.principal_type AND scope_grants.principal_id = membership.principal_id AND scope_grants.principal_authority = membership.principal_authority AND (scope_grants.organization_id = o.id OR scope_grants.tenant_id = o.tenant_id)").
		Where(squirrel.Eq{
			"membership.principal_type":      "user",
			"membership.principal_id":        userID,
			"membership.principal_authority": "marble",
		}).
		GroupBy("o.id", "o.name", "o.tenant_id", "o.environment").
		OrderBy("o.name")

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.OrganizationMembership, error) {
		var membership models.OrganizationMembership
		var environment string
		var roles []string
		if err := row.Scan(
			&membership.Organization.Id,
			&membership.Organization.Name,
			&membership.Organization.TenantId,
			&environment,
			&roles,
		); err != nil {
			return models.OrganizationMembership{}, fmt.Errorf("scanning user organization: %w", err)
		}
		membership.Organization.Environment = models.ParseOrganizationEnvironment(environment)
		for _, role := range roles {
			membership.Roles = append(membership.Roles, models.RoleFromString(role))
		}
		return membership, nil
	})
}
