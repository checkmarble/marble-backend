package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityInboxes struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityInboxes) ReadInbox(i models.Inbox) error {
	// org admins can read all inboxes
	err := e.Permission(models.INBOX_EDITOR)
	if err == nil {
		return errors.Join(err, e.ReadOrganization(i.OrganizationId))
	}

	// any other user can read an inbox if he is a member of the inbox
	for _, user := range i.InboxUsers {
		if user.UserId == string(e.Credentials.ActorIdentity.UserId) {
			return nil
		}
	}
	return errors.Wrap(models.ForbiddenError, "User does not have access to this inbox")
}

func (e *EnforceSecurityInboxes) CreateInbox(i models.CreateInboxInput) error {
	// Only org admins can create inboxes
	return errors.Join(e.Permission(models.INBOX_EDITOR), e.ReadOrganization(i.OrganizationId))
}

func (e *EnforceSecurityInboxes) ReadInboxUser(
	inboxUser models.InboxUser, actorInboxUsers []models.InboxUser, organizationId string,
) error {
	// org admins can read all inbox users
	err := e.Permission(models.INBOX_EDITOR)
	if err == nil {
		return errors.Join(err, e.ReadOrganization(organizationId))
	}

	// any other user can read an inbox user if he is a member of the inbox
	for _, user := range actorInboxUsers {
		if user.Id == inboxUser.Id {
			return nil
		}
	}
	return errors.Wrap(models.ForbiddenError, "User does not have access to this inbox user")
}

func (e *EnforceSecurityInboxes) CreateInboxUser(
	i models.CreateInboxUserInput, actorInboxUsers []models.InboxUser, organizationId string,
) error {
	// org admins can create users in all inboxes
	err := e.Permission(models.INBOX_EDITOR)
	if err == nil {
		return errors.Join(err, e.ReadOrganization(organizationId))
	}

	// any other user can create an inbox user if he is an admin of the inbox
	for _, user := range actorInboxUsers {
		if user.InboxId == i.InboxId && user.Role == models.InboxUserRoleAdmin {
			return nil
		}
	}
	return errors.Wrap(models.ForbiddenError, "User cannot create a new member in this inbox")
}
