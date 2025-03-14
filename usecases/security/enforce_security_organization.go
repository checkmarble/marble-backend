package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityOrganization interface {
	EnforceSecurity
	ListOrganization() error
	CreateOrganization() error
	EditOrganization(org models.Organization) error
	DeleteOrganization() error
	ReadDataModel() error
	WriteDataModel(organizationId string) error
	WriteDataModelIndexes(organizationId string) error
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

func (e *EnforceSecurityOrganizationImpl) EditOrganization(org models.Organization) error {
	return errors.Join(
		e.Permission(models.ORGANIZATIONS_UPDATE),
		e.ReadOrganization(org.Id),
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

func (e *EnforceSecurityOrganizationImpl) WriteDataModelIndexes(organizationId string) error {
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(organizationId),
	)
}
