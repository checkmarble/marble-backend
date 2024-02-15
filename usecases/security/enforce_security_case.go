package security

import (
	"github.com/checkmarble/marble-backend/models"

	"github.com/cockroachdb/errors"
)

type EnforceSecurityCase interface {
	EnforceSecurity
	ReadOrUpdateCase(c models.Case, availableInboxIds []string) error
	CreateCase(input models.CreateCaseAttributes, availableInboxIds []string) error
}

type EnforceSecurityCaseImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityCaseImpl) ReadOrUpdateCase(c models.Case, availableInboxIds []string) error {
	err := errors.Wrap(models.ForbiddenError, "User does not have access to cases' inbox")
	for _, inboxId := range availableInboxIds {
		if inboxId == c.InboxId {
			err = nil
			break
		}
	}
	return errors.Join(e.Permission(models.CASE_READ_WRITE),
		e.ReadOrganization(c.OrganizationId), err)
}

func (e *EnforceSecurityCaseImpl) CreateCase(input models.CreateCaseAttributes, availableInboxIds []string) error {
	err := errors.Wrap(models.ForbiddenError, "User does not have access to cases' inbox")
	for _, inboxId := range availableInboxIds {
		if inboxId == input.InboxId {
			err = nil
			break
		}
	}
	return errors.Join(e.Permission(models.CASE_READ_WRITE),
		e.ReadOrganization(input.OrganizationId), err)
}
