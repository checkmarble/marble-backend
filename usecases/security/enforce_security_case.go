package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"

	"github.com/cockroachdb/errors"
)

type EnforceSecurityCase interface {
	EnforceSecurity
	ReadOrUpdateCase(c models.CaseMetadata, availableInboxIds []uuid.UUID) error
	CreateCase(input models.CreateCaseAttributes, availableInboxIds []uuid.UUID) error
}

type EnforceSecurityCaseImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func EnforceSecurityCaseForUser(user models.User) *EnforceSecurityCaseImpl {
	creds := models.NewCredentialWithUser(user)

	return &EnforceSecurityCaseImpl{
		EnforceSecurity: NewEnforceSecurity(creds),
		Credentials:     creds,
	}
}

func (e *EnforceSecurityCaseImpl) ReadOrUpdateCase(c models.CaseMetadata, availableInboxIds []uuid.UUID) error {
	err := errors.Wrap(models.ForbiddenError, "User does not have access to case's inbox")
	for _, inboxId := range availableInboxIds {
		if inboxId == c.InboxId {
			err = nil
			break
		}
	}
	return errors.Join(e.Permission(models.CASE_READ_WRITE),
		e.ReadOrganization(c.OrganizationId), err)
}

func (e *EnforceSecurityCaseImpl) CreateCase(input models.CreateCaseAttributes, availableInboxIds []uuid.UUID) error {
	err := errors.Wrap(models.ForbiddenError, "User does not have access to case's inbox")
	for _, inboxId := range availableInboxIds {
		if inboxId == input.InboxId {
			err = nil
			break
		}
	}
	return errors.Join(e.Permission(models.CASE_READ_WRITE),
		e.ReadOrganization(input.OrganizationId), err)
}
