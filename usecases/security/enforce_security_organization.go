package security

import (
	"errors"
	"marble/marble-backend/models"
)

type EnforceSecurityOrganization interface {
	EnforceSecurity
	ListOrganization() error
	CreateOrganization() error
	DeleteOrganization() error
	ReadOrganizationApiKeys(organizationId string) error
	ReadDataModel() error
	WriteDataModel() error
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

func (e *EnforceSecurityOrganizationImpl) ReadOrganizationApiKeys(organizationId string) error {
	return errors.Join(
		e.Permission(models.APIKEY_READ),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurityOrganizationImpl) ReadDataModel() error {
	return errors.Join(
		e.Permission(models.DATA_MODEL_READ),
	)
}

func (e *EnforceSecurityOrganizationImpl) WriteDataModel() error {
	return errors.Join(
		e.Permission(models.DATA_MODEL_WRITE),
	)
}