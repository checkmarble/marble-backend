package security

import (
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"

	"github.com/cockroachdb/errors"
)

type EnforceSecurityUser interface {
	EnforceSecurity
	ReadUser(user models.User) error
	CreateUser(input models.CreateUser) error
	UpdateUser(targetUser models.User, updateUser models.UpdateUser) error
	DeleteUser(user models.User) error
	ListUsers(organizationId *uuid.UUID) error
	ListTenantUsers(organizationId uuid.UUID) error
	ManageOrganizationGrant(organizationId uuid.UUID, targetUser models.User) error
	GrantOrganizationRoleBindings(current, next []models.RoleBinding) error
	ManageRoles() error
}

type EnforceSecurityUserImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityUserImpl) ReadUser(user models.User) error {
	// Any user can list the users of their own organization, with basic information on their identity and level.
	// Currently required for the front, to be reworked if necessary.
	return errors.Join(
		e.Permission(models.MARBLE_USER_READ),
		e.ReadOrganization(user.OrganizationId),
	)
}

func (e *EnforceSecurityUserImpl) CreateUser(input models.CreateUser) error {
	roles := models.RoleNames(input.RoleBindings)

	if slices.Contains(roles, models.MARBLE_ADMIN) && !e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Wrap(
			models.ForbiddenError,
			"only marble admins can create marble admins",
		)
	}

	// should already be handled by the fact that only the ADMIN & MARBLE_ADMIN roles have the
	// MARBLE_USER_CREATE permission, but make double sure
	if slices.Contains(roles, models.ADMIN) &&
		!e.Credentials.HasRole(models.ADMIN, models.MARBLE_ADMIN) {
		return errors.Wrap(
			models.ForbiddenError,
			"only org admins and marble admins can create org admins",
		)
	}

	if err := enforceCanGrant(e, grantedCustomRolePermissions(nil, input.RoleBindings)); err != nil {
		return err
	}

	return errors.Join(
		e.Permission(models.MARBLE_USER_CREATE),
		e.ReadOrganization(input.OrganizationId),
	)
}

func (e *EnforceSecurityUserImpl) UpdateUser(targetUser models.User, updateUser models.UpdateUser) error {
	var updatedRoles []models.Role

	if updateUser.RoleBindings != nil {
		updatedRoles = models.RoleNames(*updateUser.RoleBindings)
	}

	// Only marble admins can grant or revoke the marble admin role. Others may
	// update the roles of a marble admin as long as they leave it in place:
	// role bindings are replaced as a whole, including the platform-scoped
	// marble admin grant.
	if updateUser.RoleBindings != nil &&
		!e.Credentials.HasRole(models.MARBLE_ADMIN) &&
		!sameBindingsOfRole(targetUser.RoleBindings, *updateUser.RoleBindings, models.MARBLE_ADMIN) {
		return errors.Wrap(
			models.BadParameterError,
			"only marble admins can grant or revoke the marble admin role")
	}

	// Fail early if current user is not an ADMIN and they try to change a user's role.
	if updateUser.RoleBindings != nil && !e.Credentials.HasRole(models.ADMIN) &&
		!e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Wrap(models.UnAuthorizedError, "only admins can change a user's role")
	}

	// An admin cannot strip their own ADMIN role.
	if updateUser.RoleBindings != nil &&
		e.Credentials.HasRole(models.ADMIN) &&
		e.Credentials.ActorIdentity.UserId == targetUser.UserId &&
		!slices.Contains(updatedRoles, models.ADMIN) {
		return errors.Wrap(models.BadParameterError, "Cannot remove yourself as an admin")
	}

	// Only org admins and marble admins can create org admins
	if updateUser.RoleBindings != nil &&
		slices.Contains(updatedRoles, models.ADMIN) &&
		!e.Credentials.HasRole(models.ADMIN, models.MARBLE_ADMIN) {
		return errors.Wrap(models.BadParameterError,
			"Only org admins and marble admins can create org admins")
	}

	// non admins can only update themselves
	if !e.Credentials.HasRole(models.MARBLE_ADMIN, models.ADMIN) &&
		e.Credentials.ActorIdentity.UserId != targetUser.UserId {
		return errors.Wrap(models.ForbiddenError, "non-admins can only update themselves")
	}

	if updateUser.RoleBindings != nil {
		granted := grantedCustomRolePermissions(targetUser.RoleBindings, *updateUser.RoleBindings)
		if err := enforceCanGrant(e, granted); err != nil {
			return err
		}
	}

	// lastly, in the most general case allow updates only on users of the same org
	return errors.Join(
		e.Permission(models.MARBLE_USER_UPDATE),
		e.ReadOrganization(targetUser.OrganizationId),
	)
}

func (e *EnforceSecurityUserImpl) DeleteUser(user models.User) error {
	return errors.Join(
		e.Permission(models.MARBLE_USER_DELETE),
		e.ReadOrganization(user.OrganizationId),
	)
}

func (e *EnforceSecurityUserImpl) ListUsers(organizationId *uuid.UUID) error {
	if e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Join(
			e.Permission(models.MARBLE_USER_LIST),
		)
	}

	if organizationId == nil {
		return errors.Wrap(models.ForbiddenError, "non-admin cannot list users without organization_id")
	}

	return errors.Join(
		e.Permission(models.MARBLE_USER_LIST),
		e.ReadOrganization(*organizationId),
	)
}

func (e *EnforceSecurityUserImpl) ListTenantUsers(organizationId uuid.UUID) error {
	if !e.Credentials.HasRole(models.ADMIN) && !e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Wrap(models.ForbiddenError, "only admins can list tenant users")
	}
	return errors.Join(
		e.Permission(models.MARBLE_USER_LIST),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurityUserImpl) ManageOrganizationGrant(organizationId uuid.UUID, targetUser models.User) error {
	if slices.Contains(models.RoleNames(targetUser.RoleBindings), models.MARBLE_ADMIN) &&
		!e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Wrap(models.ForbiddenError, "only marble admins can manage grants for marble admins")
	}
	if !e.Credentials.HasRole(models.ADMIN) && !e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Wrap(models.ForbiddenError, "only admins can manage organization grants")
	}
	return errors.Join(
		e.Permission(models.MARBLE_USER_UPDATE),
		e.ReadOrganization(organizationId),
	)
}

// GrantOrganizationRoleBindings only lets principals give a user, in an
// organization of its tenant, custom roles whose permissions they hold.
// Bindings left unchanged are not checked again.
func (e *EnforceSecurityUserImpl) GrantOrganizationRoleBindings(current, next []models.RoleBinding) error {
	return enforceCanGrant(e, grantedCustomRolePermissions(current, next))
}

func (e *EnforceSecurityUserImpl) ManageRoles() error {
	return errors.Join(
		e.Permission(models.MANAGE_ROLES),
		e.ReadOrganization(e.OrgId()),
	)
}

// sameBindingsOfRole reports whether both sets hold equivalent bindings of the
// given role.
func sameBindingsOfRole(current, next []models.RoleBinding, role models.Role) bool {
	currentOfRole := slices.DeleteFunc(slices.Clone(current), func(b models.RoleBinding) bool { return b.Role != role })
	nextOfRole := slices.DeleteFunc(slices.Clone(next), func(b models.RoleBinding) bool { return b.Role != role })

	if len(currentOfRole) != len(nextOfRole) {
		return false
	}

	matched := make([]bool, len(currentOfRole))
	for _, binding := range nextOfRole {
		found := false
		for idx, candidate := range currentOfRole {
			if !matched[idx] && candidate.Equivalent(binding) {
				matched[idx] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
