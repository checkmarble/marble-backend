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
	ListTenantUsersWithDirectOrganizationGrant(ctx context.Context, exec Executor, tenantID, organizationID uuid.UUID) ([]models.OrganizationUserGrant, error)
	ListTenantUsersWithoutOrganizationAccess(ctx context.Context, exec Executor, tenantID, organizationID uuid.UUID) ([]models.OrganizationUserGrant, error)
	ReassignTenantGrants(ctx context.Context, tx Transaction, targetId uuid.UUID, sourceIds []uuid.UUID) error
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
			membership.Roles = append(membership.Roles, models.Role(role))
		}
		return membership, nil
	})
}

// ListTenantUsersWithDirectOrganizationGrant lists the users of a tenant that
// have role bindings in one of its organizations, with those bindings.
func (repo *MarbleDbRepository) ListTenantUsersWithDirectOrganizationGrant(ctx context.Context, exec Executor, tenantID, organizationID uuid.UUID) ([]models.OrganizationUserGrant, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectTenantUsers(tenantID).
		Where(`EXISTS (
			SELECT 1 FROM active_grants g
			WHERE g.principal_type = 'user'
				AND g.principal_id = u.id::text
				AND g.principal_authority = 'marble'
				AND g.organization_id = ?
		)`, organizationID)

	users, err := listOrganizationUserGrants(ctx, exec, query)
	if err != nil {
		return nil, err
	}

	bindings, err := repo.listUsersOrganizationRoleBindings(ctx, exec, organizationID,
		pure_utils.Map(users, func(u models.OrganizationUserGrant) string { return string(u.User.UserId) }))
	if err != nil {
		return nil, err
	}

	for idx := range users {
		users[idx].RoleBindings = bindings[string(users[idx].User.UserId)]
	}

	return users, nil
}

// ListTenantUsersWithoutOrganizationAccess lists the users of a tenant that have
// no access to one of its organizations, neither through a role binding in it
// nor through a tenant grant.
func (repo *MarbleDbRepository) ListTenantUsersWithoutOrganizationAccess(ctx context.Context, exec Executor, tenantID, organizationID uuid.UUID) ([]models.OrganizationUserGrant, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectTenantUsers(tenantID).
		Where(`NOT EXISTS (
			SELECT 1 FROM active_grants g
			WHERE g.principal_type = 'user'
				AND g.principal_id = u.id::text
				AND g.principal_authority = 'marble'
				AND (g.organization_id = ? OR g.tenant_id = ?)
		)`, organizationID, tenantID)

	return listOrganizationUserGrants(ctx, exec, query)
}

func selectTenantUsers(tenantID uuid.UUID) squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(
			"u.id", "u.email", "u.role", "u.organization_id", "u.first_name", "u.last_name",
			"u.deleted_at", "u.ai_assist_enabled", "u.picture",
		).
		From("users u").
		Join("organizations home_org ON home_org.id = u.organization_id").
		Where(squirrel.Eq{"home_org.tenant_id": tenantID}).
		Where("u.deleted_at IS NULL").
		Where("home_org.deleted_at IS NULL").
		OrderBy("u.id")
}

func listOrganizationUserGrants(ctx context.Context, exec Executor, query squirrel.SelectBuilder) ([]models.OrganizationUserGrant, error) {
	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.OrganizationUserGrant, error) {
		var user dbmodels.DBUserResult
		if err := row.Scan(
			&user.Id,
			&user.Email,
			&user.Role,
			&user.OrganizationId,
			&user.FirstName,
			&user.LastName,
			&user.DeletedAt,
			&user.AiAssistEnabled,
			&user.Picture,
		); err != nil {
			return models.OrganizationUserGrant{}, fmt.Errorf("scanning tenant user: %w", err)
		}
		adaptedUser, err := dbmodels.AdaptUser(user)
		if err != nil {
			return models.OrganizationUserGrant{}, err
		}
		return models.OrganizationUserGrant{User: adaptedUser}, nil
	})
}

func (repo *MarbleDbRepository) ReassignTenantGrants(ctx context.Context, tx Transaction, targetId uuid.UUID, sourceIds []uuid.UUID) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	tenantIds := append([]uuid.UUID{targetId}, sourceIds...)
	grantQuery := NewQueryBuilder().
		Select("id", "principal_type", "principal_id", "principal_authority", "role").
		From("grants").
		Where(squirrel.And{
			squirrel.Eq{"revoked_at": nil},
			squirrel.Eq{"tenant_id": tenantIds},
		}).
		OrderBy("id").
		Suffix("FOR UPDATE")

	type principalKey struct {
		principalType      string
		principalId        string
		principalAuthority string
	}
	type grantKey struct {
		principalKey
		role string
	}

	// Principals may hold several roles on a tenant, so only identical
	// (principal, role) grants collide when tenants are merged.
	duplicates := make([]uuid.UUID, 0)
	seen := make(map[grantKey]uuid.UUID)
	err := ForEachRow(ctx, tx, grantQuery, func(row pgx.CollectableRow) error {
		var (
			id        uuid.UUID
			role      string
			principal principalKey
		)
		if err := row.Scan(
			&id,
			&principal.principalType,
			&principal.principalId,
			&principal.principalAuthority,
			&role,
		); err != nil {
			return fmt.Errorf("scanning tenant grant: %w", err)
		}

		key := grantKey{principalKey: principal, role: role}
		if previousId, ok := seen[key]; ok && previousId != id {
			duplicates = append(duplicates, id)
		} else {
			seen[key] = id
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(duplicates) > 0 {
		if err := ExecBuilder(ctx, tx, NewQueryBuilder().
			Update("grants").
			Set("revoked_at", squirrel.Expr("now()")).
			Where(squirrel.Eq{"id": duplicates})); err != nil {
			return err
		}
	}
	return ExecBuilder(ctx, tx, NewQueryBuilder().
		Update("grants").
		Set("tenant_id", targetId).
		Where(squirrel.And{
			squirrel.Eq{"tenant_id": sourceIds},
			squirrel.Eq{"revoked_at": nil},
		}))
}
