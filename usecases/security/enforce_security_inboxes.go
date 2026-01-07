package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityInboxes struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e EnforceSecurityInboxes) ReadInbox(i models.Inbox) error {
	// org admins can read all inboxes
	err := e.Permission(models.INBOX_EDITOR)
	if err == nil {
		orgId, _ := uuid.Parse(i.OrganizationId)
		return errors.Join(err, e.ReadOrganization(orgId))
	}

	// any other user can read an inbox if he is a member of the inbox
	actorUserIdStr := string(e.Credentials.ActorIdentity.UserId)
	for _, user := range i.InboxUsers {
		if user.UserId.String() == actorUserIdStr {
			return nil
		}
	}
	return errors.Wrap(models.ForbiddenError, "User does not have access to this inbox")
}

func (e EnforceSecurityInboxes) ReadInboxMetadata(inbox models.Inbox) error {
	orgId, _ := uuid.Parse(inbox.OrganizationId)
	return e.ReadOrganization(orgId)
}

func (e EnforceSecurityInboxes) CreateInbox(organizationId string) error {
	// Only org admins can create inboxes
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(e.Permission(models.INBOX_EDITOR), e.ReadOrganization(orgId))
}

func (e EnforceSecurityInboxes) UpdateInbox(inbox models.Inbox) error {
	// Inbox admins are allowed to update the inbox, even if they are not organization admins
	actorUserIdStr := string(e.Credentials.ActorIdentity.UserId)
	for _, inboxMember := range inbox.InboxUsers {
		if inboxMember.UserId.String() == actorUserIdStr &&
			inboxMember.Role == models.InboxUserRoleAdmin {
			return nil
		}
	}

	orgId, _ := uuid.Parse(inbox.OrganizationId)
	return errors.Join(e.Permission(models.INBOX_EDITOR),
		e.ReadOrganization(orgId))
}

func (e EnforceSecurityInboxes) ReadInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error {
	// org admins can read all inbox users
	err := e.Permission(models.INBOX_EDITOR)
	if err == nil {
		orgId, _ := uuid.Parse(inboxUser.OrganizationId)
		return errors.Join(err, e.ReadOrganization(orgId))
	}

	// any other user can read an inbox user if he is a member of the inbox
	for _, user := range actorInboxUsers {
		if user.InboxId == inboxUser.InboxId {
			return nil
		}
	}
	return errors.Wrap(models.ForbiddenError, "User does not have access to this inbox user")
}

func (e EnforceSecurityInboxes) CreateInboxUser(
	i models.CreateInboxUserInput, actorInboxUsers []models.InboxUser, targetInbox models.Inbox, targetUser models.User,
) error {
	organizationId := e.Credentials.OrganizationId
	if targetUser.OrganizationId != organizationId {
		return errors.Wrap(models.ForbiddenError, "Target user does not belong to the right organization")
	}
	targetInboxOrgId, _ := uuid.Parse(targetInbox.OrganizationId)
	if targetInboxOrgId != organizationId {
		return errors.Wrap(models.ForbiddenError, "Target inbox does not belong to the right organization")
	}

	// org admins can create users in all inboxes
	err := e.Permission(models.INBOX_EDITOR)
	if err == nil {
		orgId, _ := uuid.Parse(targetInbox.OrganizationId)
		return errors.Join(err, e.ReadOrganization(orgId))
	}

	// any other user can create an inbox user if he is an admin of the inbox
	for _, user := range actorInboxUsers {
		if user.InboxId == i.InboxId && user.Role == models.InboxUserRoleAdmin {
			return nil
		}
	}
	return errors.Wrap(models.ForbiddenError, "User cannot create a new member in this inbox")
}

func (e EnforceSecurityInboxes) UpdateInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error {
	// org admins can update all inbox users
	err := e.Permission(models.INBOX_EDITOR)
	if err == nil {
		orgId, _ := uuid.Parse(inboxUser.OrganizationId)
		return errors.Join(err, e.ReadOrganization(orgId))
	}

	// any other user can update an inbox user if he is an admin of the inbox
	for _, user := range actorInboxUsers {
		if user.InboxId == inboxUser.InboxId && user.Role == models.InboxUserRoleAdmin {
			return nil
		}
	}
	return errors.Wrap(models.ForbiddenError, "User cannot update this inbox user")
}
