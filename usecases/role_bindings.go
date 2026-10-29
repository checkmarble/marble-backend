package usecases

import (
	"context"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type customRoleGetter interface {
	GetRoleBySlug(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, slug models.Role) (models.RbacRole, error)
}

// resolveRoleBindings validates role bindings and resolves their permissions.
// Native roles must be among nativeRoles, and custom roles are looked up in
// the organization.
func resolveRoleBindings(
	ctx context.Context,
	exec repositories.Executor,
	roles customRoleGetter,
	orgId uuid.UUID,
	bindings []models.RoleBinding,
	nativeRoles []models.Role,
) ([]models.RoleBinding, error) {
	resolved := slices.Clone(bindings)

	for idx := range resolved {
		binding := &resolved[idx]
		if binding.Role == "" {
			return nil, errors.Wrap(models.BadParameterError, "role binding must reference a role")
		}

		if err := binding.Conditions.Validate(); err != nil {
			return nil, err
		}

		if !binding.Role.IsCustom() {
			if !slices.Contains(nativeRoles, binding.Role) {
				return nil, errors.Wrapf(models.BadParameterError, "role %s cannot be bound here", binding.Role)
			}
			binding.CustomRoleId = nil
			binding.Permissions = binding.Role.Permissions()
			continue
		}
		if !binding.Role.IsValidCustom() {
			return nil, errors.Wrap(models.BadParameterError, "invalid custom role slug")
		}

		role, err := roles.GetRoleBySlug(ctx, exec, orgId, binding.Role)
		if err != nil {
			return nil, err
		}
		binding.CustomRoleId = &role.Id
		binding.Permissions = role.Permissions
	}

	return resolved, nil
}
