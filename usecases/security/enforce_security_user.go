package security

import (
	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"
)

type EnforceSecurityUser interface {
	EnforceSecurity
	ReadUser(user models.User) error
	CreateUser(input models.CreateUser) error
	UpdateUser(targetUser models.User, updateUser models.UpdateUser) error
	DeleteUser(user models.User) error
	ListUsers(organizationId *string) error
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
	if input.Role == models.MARBLE_ADMIN && e.Credentials.Role != models.MARBLE_ADMIN {
		return errors.Wrap(
			models.ForbiddenError,
			"only marble admins can create marble admins",
		)
	}

	// should already be handled by the fact that only the ADMIN & MARBLE_ADMIN roles have the
	// MARBLE_USER_CREATE permission, but make double sure
	if input.Role == models.ADMIN &&
		!(e.Credentials.Role == models.ADMIN || e.Credentials.Role == models.MARBLE_ADMIN) {
		return errors.Wrap(
			models.ForbiddenError,
			"only org admins and marble admins can create org admins",
		)
	}

	if input.PartnerId != nil && (e.Credentials.Role != models.MARBLE_ADMIN ||
		e.Credentials.PartnerId == nil ||
		*e.Credentials.PartnerId != *input.PartnerId) {
		return errors.Wrap(
			models.ForbiddenError,
			"only marble admins can create users with partner_id",
		)
	}

	return errors.Join(
		e.Permission(models.MARBLE_USER_CREATE),
		e.ReadOrganization(input.OrganizationId),
	)
}

func (e *EnforceSecurityUserImpl) UpdateUser(targetUser models.User, updateUser models.UpdateUser) error {
	// Only marble admins can create marble admins
	if updateUser.Role != nil &&
		*updateUser.Role == models.MARBLE_ADMIN &&
		e.Credentials.Role != models.MARBLE_ADMIN {
		return errors.Wrap(
			models.BadParameterError,
			"only marble admins can create marble admins")
	}

	// Only org admins and marble admins can create org admins
	if updateUser.Role != nil &&
		*updateUser.Role != models.ADMIN &&
		e.Credentials.Role == models.ADMIN {
		return errors.Wrap(models.BadParameterError, "Cannot remove yourself as an admin")
	}

	// Only org admins and marble admins can create org admins
	if updateUser.Role != nil &&
		*updateUser.Role == models.ADMIN &&
		!(e.Credentials.Role == models.ADMIN || e.Credentials.Role == models.MARBLE_ADMIN) {
		return errors.Wrap(models.BadParameterError,
			"Only org admins and marble admins can create org admins")
	}

	// non admins can only update themselves
	if (e.Credentials.Role != models.MARBLE_ADMIN && e.Credentials.Role != models.ADMIN) &&
		e.Credentials.ActorIdentity.UserId != targetUser.UserId {
		return errors.Wrap(models.ForbiddenError, "non-admins can only update themselves")
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

func (e *EnforceSecurityUserImpl) ListUsers(organizationId *string) error {
	if e.Credentials.Role == models.MARBLE_ADMIN {
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
