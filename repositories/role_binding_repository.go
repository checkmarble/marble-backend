package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) ListUserRoleBindings(ctx context.Context, exec Executor, orgId uuid.UUID, userId string) ([]models.RoleBinding, error) {
	return repo.listRoleBindings(ctx, exec, orgId, dbmodels.GrantPrincipalUser, userId)
}

func (repo *MarbleDbRepository) ListApiKeyRoleBindings(ctx context.Context, exec Executor, orgId uuid.UUID, apiKeyId string) ([]models.RoleBinding, error) {
	return repo.listRoleBindings(ctx, exec, orgId, dbmodels.GrantPrincipalApiKey, apiKeyId)
}

// ReplaceUserRoleBindings replaces the role bindings of a user in its home
// organization, including its platform bindings.
func (repo *MarbleDbRepository) ReplaceUserRoleBindings(
	ctx context.Context,
	tx Transaction,
	orgId uuid.UUID,
	userId string,
	bindings []models.RoleBinding,
) error {
	return repo.replaceRoleBindings(ctx, tx, homeScope(orgId), dbmodels.GrantPrincipalUser, userId, bindings)
}

// ReplaceUserOrganizationRoleBindings replaces the role bindings of a user in
// an organization that may not be its home organization. Its platform bindings
// and its bindings in other organizations are left untouched.
func (repo *MarbleDbRepository) ReplaceUserOrganizationRoleBindings(
	ctx context.Context,
	tx Transaction,
	orgId uuid.UUID,
	userId string,
	bindings []models.RoleBinding,
) error {
	if orgId == uuid.Nil {
		return fmt.Errorf("organization role bindings need an organization: %w", models.BadParameterError)
	}

	// Platform roles are managed with the bindings of the user's home
	// organization only.
	for _, binding := range bindings {
		if binding.Role == models.MARBLE_ADMIN {
			return fmt.Errorf("role %s cannot be bound to an organization: %w", binding.Role, models.BadParameterError)
		}
	}

	return repo.replaceRoleBindings(ctx, tx, organizationScope(orgId), dbmodels.GrantPrincipalUser, userId, bindings)
}

func (repo *MarbleDbRepository) ReplaceApiKeyRoleBindings(
	ctx context.Context,
	tx Transaction,
	orgId uuid.UUID,
	apiKeyId string,
	bindings []models.RoleBinding,
) error {
	return repo.replaceRoleBindings(ctx, tx, homeScope(orgId), dbmodels.GrantPrincipalApiKey, apiKeyId, bindings)
}

// roleBindingsQuery selects the role bindings of one principal, or of several
// when given a list of IDs.
func roleBindingsQuery(principalType string, principalId any) squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectRoleBindingColumns...).
		From(dbmodels.TABLE_GRANTS+" g").
		Where(squirrel.Eq{
			"g.principal_type":      principalType,
			"g.principal_id":        principalId,
			"g.principal_authority": dbmodels.GrantAuthorityMarble,
			"g.revoked_at":          nil,
		}).
		OrderBy("g.created_at", "g.id")
}

// managedScope restricts a grants query to the grants managed through role
// bindings for the given organization: the organization's own grants, and
// platform grants (neither organization nor tenant), which apply everywhere.
func managedScope(prefix string, orgId uuid.UUID) squirrel.Sqlizer {
	platform := squirrel.Eq{
		prefix + "organization_id": nil,
		prefix + "tenant_id":       nil,
	}

	if orgId == uuid.Nil {
		return platform
	}

	return squirrel.Or{squirrel.Eq{prefix + "organization_id": orgId}, platform}
}

// roleBindingScope selects the grants a replacement of role bindings manages.
type roleBindingScope struct {
	orgId uuid.UUID
	// platform includes platform bindings, which are managed with the bindings
	// of the principal's home organization.
	platform bool
}

func homeScope(orgId uuid.UUID) roleBindingScope {
	return roleBindingScope{orgId: orgId, platform: true}
}

func organizationScope(orgId uuid.UUID) roleBindingScope {
	return roleBindingScope{orgId: orgId}
}

func (s roleBindingScope) where(prefix string) squirrel.Sqlizer {
	if s.platform {
		return managedScope(prefix, s.orgId)
	}

	return squirrel.Eq{prefix + "organization_id": s.orgId}
}

func (repo *MarbleDbRepository) listRoleBindings(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	principalType, principalId string,
) ([]models.RoleBinding, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return repo.listScopedRoleBindings(ctx, exec, homeScope(orgId), principalType, principalId)
}

func (repo *MarbleDbRepository) listScopedRoleBindings(
	ctx context.Context,
	exec Executor,
	scope roleBindingScope,
	principalType, principalId string,
) ([]models.RoleBinding, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(ctx, exec,
		roleBindingsQuery(principalType, principalId).Where(scope.where("g.")),
		dbmodels.AdaptRoleBinding)
}

// listUsersOrganizationRoleBindings returns the role bindings of several users
// in one organization, which may not be their home organization, keyed by
// user ID.
func (repo *MarbleDbRepository) listUsersOrganizationRoleBindings(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	userIds []string,
) (map[string][]models.RoleBinding, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	if len(userIds) == 0 {
		return map[string][]models.RoleBinding{}, nil
	}

	bindings, err := SqlToListOfModels(ctx, exec,
		roleBindingsQuery(dbmodels.GrantPrincipalUser, userIds).Where(organizationScope(orgId).where("g.")),
		dbmodels.AdaptRoleBinding)
	if err != nil {
		return nil, err
	}

	byUser := make(map[string][]models.RoleBinding, len(userIds))
	for _, binding := range bindings {
		if binding.UserId != nil {
			byUser[string(*binding.UserId)] = append(byUser[string(*binding.UserId)], binding)
		}
	}

	return byUser, nil
}

func (repo *MarbleDbRepository) replaceRoleBindings(
	ctx context.Context,
	tx Transaction,
	scope roleBindingScope,
	principalType, principalId string,
	bindings []models.RoleBinding,
) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	orgId := scope.orgId

	existing, err := repo.listScopedRoleBindings(ctx, tx, scope, principalType, principalId)
	if err != nil {
		return err
	}

	kept := make(map[uuid.UUID]bool, len(existing))
	toInsert := make([]models.RoleBinding, 0, len(bindings))
	seenRoles := make(map[models.Role]bool, len(bindings))

	for _, binding := range bindings {
		if err := validateRoleBinding(binding); err != nil {
			return err
		}
		if seenRoles[binding.Role] {
			return fmt.Errorf("role %s is bound more than once: %w", binding.Role, models.BadParameterError)
		}
		seenRoles[binding.Role] = true

		matched := false
		for _, current := range existing {
			if !kept[current.Id] && sameRoleBinding(current, binding) {
				kept[current.Id] = true
				matched = true
				break
			}
		}
		if !matched {
			toInsert = append(toInsert, binding)
		}
	}

	revoked := make([]uuid.UUID, 0, len(existing))
	for _, current := range existing {
		if !kept[current.Id] {
			revoked = append(revoked, current.Id)
		}
	}

	if len(revoked) > 0 {
		if err := ExecBuilder(ctx, tx, NewQueryBuilder().
			Update(dbmodels.TABLE_GRANTS).
			Set("revoked_at", squirrel.Expr("now()")).
			Where(squirrel.Eq{"id": revoked})); err != nil {
			return err
		}
	}

	for _, binding := range toInsert {
		var organizationId any

		// MARBLE_ADMIN is a platform role and is never bound to an organization.
		if orgId != uuid.Nil && binding.Role != models.MARBLE_ADMIN {
			organizationId = orgId
		}

		if organizationId == nil && binding.Role != models.MARBLE_ADMIN {
			return fmt.Errorf("cannot bind an organization role to a principal without an organization: %w",
				models.BadParameterError)
		}

		if err := ExecBuilder(ctx, tx, NewQueryBuilder().
			Insert(dbmodels.TABLE_GRANTS).
			Columns(
				"id",
				"principal_type",
				"principal_id",
				"principal_authority",
				"organization_id",
				"role",
			).
			Values(
				pure_utils.NewId(),
				principalType,
				principalId,
				dbmodels.GrantAuthorityMarble,
				organizationId,
				binding.Role.String(),
			)); err != nil {
			return err
		}
	}

	return nil
}

func validateRoleBinding(binding models.RoleBinding) error {
	if binding.Role == "" {
		return fmt.Errorf("role binding must reference a role: %w", models.BadParameterError)
	}

	return nil
}

// sameRoleBinding reports whether an existing grant already represents the
// given binding, in which case it is kept rather than revoked and recreated.
func sameRoleBinding(current, next models.RoleBinding) bool {
	return current.Equivalent(next)
}
