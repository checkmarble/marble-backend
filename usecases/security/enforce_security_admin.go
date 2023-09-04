package security

import (
	"errors"
	"marble/marble-backend/models"
)

type EnforceSecurityAdmin interface {
	EnforceSecurity
	CreateUser() error
	DeleteUser() error
	ListUser() error
}

type EnforceSecurityAdminImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityAdminImpl) CreateUser() error {
	return errors.Join(
		e.Permission(models.MARBLE_USER_CREATE),
	)
}

func (e *EnforceSecurityAdminImpl) DeleteUser() error {
	return errors.Join(
		e.Permission(models.MARBLE_USER_DELETE),
	)
}

func (e *EnforceSecurityAdminImpl) ListUser() error {
	return errors.Join(
		e.Permission(models.MARBLE_USER_LIST),
	)
}