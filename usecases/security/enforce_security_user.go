package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"slices"

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
	ListUserGrants() error
}

type EnforceSecurityUserImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityUserImpl) ReadUser(user models.User) error {
	if err := e.Permission(models.MARBLE_USER_READ); err != nil {
		return err
	}
	if e.Credentials.HasRole(models.MARBLE_ADMIN) || e.Credentials.ActorIdentity.UserId == user.UserId {
		return nil
	}
	if e.Credentials.HasRole(models.TENANT_ADMIN) && e.Credentials.TenantId != uuid.Nil && e.Credentials.TenantId == user.TenantId {
		return nil
	}
	if e.Credentials.OrganizationId != uuid.Nil && slices.Contains(user.OrganizationIds, e.Credentials.OrganizationId) {
		return nil
	}
	return models.ForbiddenError
}

func (e *EnforceSecurityUserImpl) CreateUser(input models.CreateUser) error {
	if e.Credentials.HasRole(models.TENANT_ADMIN) && input.Role != models.MARBLE_ADMIN && input.TenantId == e.Credentials.TenantId && input.TenantId != uuid.Nil {
		return e.Permission(models.MARBLE_USER_CREATE)
	}
	if input.Role == models.MARBLE_ADMIN && !e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Wrap(
			models.ForbiddenError,
			"only marble admins can create marble admins",
		)
	}

	// should already be handled by the fact that only the ADMIN & MARBLE_ADMIN roles have the
	// MARBLE_USER_CREATE permission, but make double sure
	if input.Role == models.ADMIN &&
		!(e.Credentials.HasRole(models.ADMIN) || e.Credentials.HasRole(models.MARBLE_ADMIN)) {
		return errors.Wrap(
			models.ForbiddenError,
			"only org admins and marble admins can create org admins",
		)
	}

	return errors.Join(
		e.Permission(models.MARBLE_USER_CREATE),
		e.ReadOrganization(input.OrganizationId),
	)
}

func (e *EnforceSecurityUserImpl) UpdateUser(targetUser models.User, updateUser models.UpdateUser) error {
	if targetUser.Role == models.MARBLE_ADMIN && !e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return models.ForbiddenError
	}
	if !e.Credentials.HasRole(models.MARBLE_ADMIN) && !e.Credentials.HasRole(models.TENANT_ADMIN) && !e.Credentials.HasRole(models.ADMIN) && e.Credentials.ActorIdentity.UserId != targetUser.UserId {
		return models.ForbiddenError
	}
	if e.Credentials.HasRole(models.ADMIN) && !e.Credentials.HasRole(models.MARBLE_ADMIN) && !e.Credentials.HasRole(models.TENANT_ADMIN) && e.Credentials.ActorIdentity.UserId != targetUser.UserId &&
		(updateUser.Email != nil || updateUser.FirstName != nil || updateUser.LastName != nil) {
		return models.ForbiddenError
	}
	if e.Credentials.HasRole(models.TENANT_ADMIN) && e.Credentials.TenantId == targetUser.TenantId && targetUser.TenantId != uuid.Nil {
		if updateUser.Role != nil && *updateUser.Role == models.MARBLE_ADMIN {
			return models.ForbiddenError
		}
		if updateUser.Role != nil && e.Credentials.ActorIdentity.UserId == targetUser.UserId {
			return models.ForbiddenError
		}
		return e.Permission(models.MARBLE_USER_UPDATE)
	}
	// Only marble admins can create marble admins
	if updateUser.Role != nil &&
		*updateUser.Role == models.MARBLE_ADMIN &&
		!e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Wrap(
			models.BadParameterError,
			"only marble admins can create marble admins")
	}

	// Fail early if current user is not an ADMIN and they try to change a user's role.
	if updateUser.Role != nil && !e.Credentials.HasRole(models.ADMIN) &&
		!e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return errors.Wrap(models.UnAuthorizedError, "only admins can change a user's role")
	}

	// An admin cannot strip their own ADMIN role.
	if updateUser.Role != nil &&
		e.Credentials.HasRole(models.ADMIN) &&
		e.Credentials.ActorIdentity.UserId == targetUser.UserId &&
		*updateUser.Role != models.ADMIN {
		return errors.Wrap(models.BadParameterError, "Cannot remove yourself as an admin")
	}

	// Only org admins and marble admins can create org admins
	if updateUser.Role != nil &&
		*updateUser.Role == models.ADMIN &&
		!(e.Credentials.HasRole(models.ADMIN) || e.Credentials.HasRole(models.MARBLE_ADMIN)) {
		return errors.Wrap(models.BadParameterError,
			"Only org admins and marble admins can create org admins")
	}

	// non admins can only update themselves
	if !e.Credentials.HasRole(models.MARBLE_ADMIN) && !e.Credentials.HasRole(models.ADMIN) &&
		e.Credentials.ActorIdentity.UserId != targetUser.UserId {
		return errors.Wrap(models.ForbiddenError, "non-admins can only update themselves")
	}

	// lastly, in the most general case allow updates only on users of the same org
	return errors.Join(
		e.Permission(models.MARBLE_USER_UPDATE),
		e.ReadUser(targetUser),
	)
}

func (e *EnforceSecurityUserImpl) DeleteUser(user models.User) error {
	if e.Credentials.ActorIdentity.UserId == user.UserId {
		return models.ForbiddenError
	}
	if e.Credentials.HasRole(models.MARBLE_ADMIN) {
		return e.Permission(models.MARBLE_USER_DELETE)
	}
	if user.Role != models.MARBLE_ADMIN && e.Credentials.HasRole(models.TENANT_ADMIN) && user.TenantId != uuid.Nil && user.TenantId == e.Credentials.TenantId {
		return e.Permission(models.MARBLE_USER_DELETE)
	}
	return models.ForbiddenError
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
	if e.Credentials.HasRole(models.TENANT_ADMIN) && targetUser.TenantId != uuid.Nil && targetUser.TenantId == e.Credentials.TenantId && targetUser.Role != models.MARBLE_ADMIN {
		return e.Permission(models.MARBLE_USER_UPDATE)
	}
	if targetUser.Role == models.MARBLE_ADMIN && !e.Credentials.HasRole(models.MARBLE_ADMIN) {
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

// ListUserGrants guards the cross-tenant view of a user's grants, which only the backoffice uses.
func (e *EnforceSecurityUserImpl) ListUserGrants() error {
	if !e.Credentials.HasRole(models.MARBLE_ADMIN) && !(e.Credentials.HasRole(models.TENANT_ADMIN) && e.Credentials.TenantId != uuid.Nil) {
		return errors.Wrap(models.ForbiddenError, "only marble admins can list user grants")
	}
	return e.Permission(models.MARBLE_USER_LIST)
}
