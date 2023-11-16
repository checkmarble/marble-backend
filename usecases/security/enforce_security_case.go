package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityCase interface {
	EnforceSecurity
	ReadCase(c models.Case) error
	CreateCase() error
	UpdateCase(c models.Case) error
}

type EnforceSecurityCaseImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityCaseImpl) ReadCase(c models.Case) error {
	return errors.Join(e.Permission(models.CASE_READ), e.ReadOrganization(c.OrganizationId))
}

func (e *EnforceSecurityCaseImpl) CreateCase() error {
	return errors.Join(e.Permission(models.CASE_CREATE))
}

func (e *EnforceSecurityCaseImpl) UpdateCase(c models.Case) error {
	return errors.Join(e.Permission(models.CASE_CREATE), e.ReadOrganization(c.OrganizationId))
}
