package security

import (
	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"
)

type EnforceSecurityUser interface {
	EnforceSecurity
	CreateUser(input models.CreateUser) error
	UpdateUser(user models.User) error
	DeleteUser(user models.User) error
	ListUser() error
}

type EnforceSecurityUserImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityUserImpl) CreateUser(input models.CreateUser) error {
	var errLevel error
	if input.Role == models.MARBLE_ADMIN && e.Credentials.Role != models.MARBLE_ADMIN {
		errLevel = errors.Wrap(
			models.ForbiddenError,
			"only marble admins can create marble admins")
	}

	var errPartner error
	if input.PartnerId != nil && (e.Credentials.Role != models.MARBLE_ADMIN ||
		e.Credentials.PartnerId == nil ||
		*e.Credentials.PartnerId != *input.PartnerId) {
		errPartner = errors.Wrap(
			models.ForbiddenError,
			"only marble admins can create users with partner_id")
	}

	return errors.Join(
		e.Permission(models.MARBLE_USER_CREATE),
		e.ReadOrganization(input.OrganizationId),
		errLevel,
		errPartner,
	)
}

func (e *EnforceSecurityUserImpl) UpdateUser(user models.User) error {
	return errors.Join(
		e.Permission(models.MARBLE_USER_CREATE), e.ReadOrganization(user.OrganizationId),
	)
}

func (e *EnforceSecurityUserImpl) DeleteUser(user models.User) error {
	return errors.Join(
		e.Permission(models.MARBLE_USER_DELETE), e.ReadOrganization(user.OrganizationId),
	)
}

func (e *EnforceSecurityUserImpl) ListUser() error {
	return errors.Join(
		e.Permission(models.MARBLE_USER_LIST),
	)
}
