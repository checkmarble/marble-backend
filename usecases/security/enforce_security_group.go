package security

import (
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityGroup interface {
	EnforceSecurity
	ManageGroups() error
	GrantGroupRoleBindings(current, next []models.RoleBinding) error
	KeepOwnAdmin(groupId uuid.UUID) error
}

type EnforceSecurityGroupImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityGroupImpl) ManageGroups() error {
	if e.OrgId() == uuid.Nil {
		return errors.Wrap(models.BadParameterError, "groups can only be managed within an organization")
	}

	return errors.Join(
		e.Permission(models.MANAGE_ROLES),
		e.ReadOrganization(e.OrgId()),
	)
}

// GrantGroupRoleBindings only lets principals give the members of a group
// role bindings they could give them directly: native roles can only be
// granted by admins, and custom roles only grant permissions the principal
// holds. Bindings left unchanged are not checked again.
func (e *EnforceSecurityGroupImpl) GrantGroupRoleBindings(current, next []models.RoleBinding) error {
	if grantsNativeRole(current, next) && !e.Credentials.HasRole(models.ADMIN, models.MARBLE_ADMIN) {
		return errors.Wrap(models.ForbiddenError, "only admins can grant native roles through a group")
	}

	return enforceCanGrant(e, grantedCustomRolePermissions(current, next))
}

// KeepOwnAdmin forbids changes to a group that could strip the actor of its
// admin role, when the group is where the actor gets that role from.
func (e *EnforceSecurityGroupImpl) KeepOwnAdmin(groupId uuid.UUID) error {
	if e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return nil
	}

	adminThroughGroup := false
	for _, binding := range e.Credentials.RoleBindings {
		if binding.Role != models.ADMIN || !binding.IsActive(e.Credentials.RoleBindingBundle) {
			continue
		}
		if binding.GroupId == nil || *binding.GroupId != groupId {
			return nil
		}
		adminThroughGroup = true
	}

	if adminThroughGroup {
		return errors.Wrap(models.BadParameterError, "cannot remove your own admin role")
	}

	return nil
}

// grantsNativeRole reports whether next holds a native role binding that has
// no equivalent in current.
func grantsNativeRole(current, next []models.RoleBinding) bool {
	return slices.ContainsFunc(next, func(binding models.RoleBinding) bool {
		return !binding.Role.IsCustom() && !slices.ContainsFunc(current, binding.Equivalent)
	})
}
