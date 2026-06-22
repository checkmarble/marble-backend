package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityOrganization interface {
	EnforceSecurity
	ListOrganization() error
	CreateOrganization() error
	EditOrganization(org models.Organization) error
	EditOrganizationScreeningProvider(org models.Organization, isManagedMarble bool) error
	DeleteOrganization() error
	ReadDataModel() error
	WriteDataModel(organizationId uuid.UUID) error
	WriteDataModelIndexes(organizationId uuid.UUID) error
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

// EditOrganizationScreeningProvider enforces the correct permissions for editing the screening provider,
// depending on whether the deployment is managed by Marble or self-hosted.
// For managed deployments, only Marble admins can change the screening provider.
// For self-hosted deployments, the credentials are used to determine the authority.
func (e *EnforceSecurityOrganizationImpl) EditOrganizationScreeningProvider(org models.Organization, isManagedMarble bool) error {
	if isManagedMarble {
		if e.Credentials.Role != models.MARBLE_ADMIN {
			return errors.Wrap(
				models.ForbiddenError,
				"only marble admins can change the screening provider on managed deployments",
			)
		}
		return nil
	}
	return e.EditOrganization(org)
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

func (e *EnforceSecurityOrganizationImpl) WriteDataModel(organizationId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.DATA_MODEL_WRITE),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurityOrganizationImpl) WriteDataModelIndexes(organizationId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.SCENARIO_CREATE),
		e.ReadOrganization(organizationId),
	)
}
