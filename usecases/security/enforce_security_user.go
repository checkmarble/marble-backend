package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityUser interface {
	EnforceSecurity
	CreateUser(organizationId string) error
	UpdateUser(user models.User) error
	DeleteUser(user models.User) error
	ListUser() error
}

type EnforceSecurityUserImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityUserImpl) CreateUser(organizationId string) error {
	return errors.Join(
		e.Permission(models.MARBLE_USER_CREATE), e.ReadOrganization(organizationId),
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
