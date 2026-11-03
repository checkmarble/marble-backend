package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

// grantedCustomRolePermissions returns the permissions of the custom role
// bindings in next that have no equivalent in current, that is the custom
// roles being granted. Bindings left unchanged are not granted again.
func grantedCustomRolePermissions(current, next []models.RoleBinding) []models.Permission {
	matched := make([]bool, len(current))
	permissions := make([]models.Permission, 0)

	for _, binding := range next {
		if !binding.Role.IsCustom() {
			continue
		}

		unchanged := false
		for idx, candidate := range current {
			if !matched[idx] && candidate.Equivalent(binding) {
				matched[idx] = true
				unchanged = true
				break
			}
		}

		if !unchanged {
			permissions = append(permissions, binding.Permissions...)
		}
	}

	return permissions
}

// enforceCanGrant only lets principals grant, through custom roles,
// permissions they hold themselves.
func enforceCanGrant(e EnforceSecurity, permissions []models.Permission) error {
	for _, permission := range permissions {
		if permission.IsPlatform() {
			return errors.Wrapf(models.ForbiddenError,
				"platform permission %s cannot be granted through custom roles", permission)
		}
	}

	if err := e.Permissions(permissions); err != nil {
		return errors.Wrap(err, "custom roles can only grant permissions you hold")
	}

	return nil
}
