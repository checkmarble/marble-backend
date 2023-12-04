package security

import (
	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"
)

type EnforceSecurityCase interface {
	EnforceSecurity
	ReadCase(c models.Case, availableInboxIds []string) error
	CreateCase() error
	UpdateCase(c models.Case) error
	CreateCaseComment(c models.Case) error
}

type EnforceSecurityCaseImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityCaseImpl) ReadCase(c models.Case, availableInboxIds []string) error {
	err := errors.Wrap(models.ForbiddenError, "User does not have access to cases' inbox")
	for _, inboxId := range availableInboxIds {
		if inboxId == c.InboxId {
			err = nil
			break
		}
	}
	return errors.Join(e.Permission(models.CASE_READ), e.ReadOrganization(c.OrganizationId), err)
}

func (e *EnforceSecurityCaseImpl) CreateCase() error {
	return errors.Join(e.Permission(models.CASE_CREATE))
}

func (e *EnforceSecurityCaseImpl) UpdateCase(c models.Case) error {
	return errors.Join(e.Permission(models.CASE_CREATE), e.ReadOrganization(c.OrganizationId))
}

func (e *EnforceSecurityCaseImpl) CreateCaseComment(c models.Case) error {
	return errors.Join(e.Permission(models.CASE_READ), e.ReadOrganization(c.OrganizationId))
}
