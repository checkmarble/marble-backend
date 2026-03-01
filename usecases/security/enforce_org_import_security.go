package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityOrgImportImpl struct {
	EnforceSecurity

	Credentials models.Credentials
}

func (e *EnforceSecurityOrgImportImpl) ImportOrg() error {
	if e.Credentials.Role != models.MARBLE_ADMIN {
		return errors.Wrap(models.UnAuthorizedError,
			"only admins can import an organization")
	}

	return nil
}

func (e *EnforceSecurityOrgImportImpl) ListOrgArchetypes() error {
	return e.Permission(models.ORG_IMPORT_ARCHETYPE_READ)
}

func (e *EnforceSecurityOrgImportImpl) ImportIntoOrg(orgId uuid.UUID) error {
	return errors.Join(e.Permission(models.ORG_IMPORT_INTO_EXISTING), e.ReadOrganization(orgId))
}

func (e *EnforceSecurityOrgImportImpl) ExportOrg(orgId uuid.UUID) error {
	return errors.Join(e.Permission(models.ORG_EXPORT), e.ReadOrganization(orgId))
}
