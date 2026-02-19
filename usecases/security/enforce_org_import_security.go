package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityOrgImportImpl struct {
	EnforceSecurity

	Credentials models.Credentials
}

func (e *EnforceSecurityOrgImportImpl) ImportOrg() error {
	if e.Credentials.Role != models.MARBLE_ADMIN {
		return errors.Wrap(models.UnAuthorizedError, "only admins can import an organization")
	}

	return nil
}

func (e *EnforceSecurityOrgImportImpl) ListOrgArchetypes() error {
	return e.Permission(models.ORG_IMPORT_READ)
}
