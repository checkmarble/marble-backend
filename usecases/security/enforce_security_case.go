package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityCase interface {
	EnforceSecurity
	ReadCase(c models.Case) error
}

type EnforceSecurityCaseImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityCaseImpl) ReadCase(c models.Case) error {
	return errors.Join(e.Permission(models.CASE_READ), e.ReadOrganization(c.OrganizationId))
}
