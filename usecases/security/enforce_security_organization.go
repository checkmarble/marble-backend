package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityOrganization interface {
	EnforceSecurity
	ListOrganization() error
	CreateOrganization() error
	DeleteOrganization() error
	ReadDataModel() error
	WriteDataModel(organizationId string) error
}

type EnforceSecurityOrganizationImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityOrganizationImpl) ListOrganization() error {
	return errors.Join(
		e.Permission(models.ORGANIZATIONS_LIST),
	)
}

func (e *EnforceSecurityOrganizationImpl) CreateOrganization() error {
	return errors.Join(
		e.Permission(models.ORGANIZATIONS_CREATE),
	)
}

func (e *EnforceSecurityOrganizationImpl) DeleteOrganization() error {
	return errors.Join(
		e.Permission(models.ORGANIZATIONS_DELETE),
	)
}

func (e *EnforceSecurityOrganizationImpl) ReadDataModel() error {
	return errors.Join(
		e.Permission(models.DATA_MODEL_READ),
	)
}

func (e *EnforceSecurityOrganizationImpl) WriteDataModel(organizationId string) error {
	return errors.Join(
		e.Permission(models.DATA_MODEL_WRITE),
		e.ReadOrganization(organizationId),
	)
}
