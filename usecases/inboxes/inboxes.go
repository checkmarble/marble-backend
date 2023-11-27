package inboxes

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type InboxRepository interface {
	GetInboxById(tx repositories.Transaction, inboxId string) (models.Inbox, error)
	ListInboxes(tx repositories.Transaction, organizationId string, inboxIds []string) ([]models.Inbox, error)
	CreateInbox(tx repositories.Transaction, createInboxAttributes models.CreateInboxInput, newInboxId string) error
	GetInboxUserById(tx repositories.Transaction, inboxUserId string) (models.InboxUser, error)
	ListInboxUsers(tx repositories.Transaction, filters models.InboxUserFilterInput) ([]models.InboxUser, error)
	CreateInboxUser(tx repositories.Transaction, createInboxUserAttributes models.CreateInboxUserInput, newInboxUserId string) error
}

type EnforceSecurityInboxes interface {
	ReadInbox(i models.Inbox) error
	CreateInbox(i models.CreateInboxInput) error
	ReadInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error
	CreateInboxUser(i models.CreateInboxUserInput, actorInboxUsers []models.InboxUser, targetInbox models.Inbox, targetUser models.User) error
}

type InboxReader struct {
	EnforceSecurity         EnforceSecurityInboxes
	OrganizationIdOfContext func() (string, error)
	InboxRepository         InboxRepository
	Credentials             models.Credentials
}

func (i *InboxReader) GetInboxById(ctx context.Context, inboxId string) (models.Inbox, error) {
	inbox, err := i.InboxRepository.GetInboxById(nil, inboxId)
	if err != nil {
		return models.Inbox{}, err
	}

	if err = i.EnforceSecurity.ReadInbox(inbox); err != nil {
		return models.Inbox{}, err
	}

	return inbox, err
}

func (i *InboxReader) ListInboxes(ctx context.Context) ([]models.Inbox, error) {
	organizationId, err := i.OrganizationIdOfContext()
	if err != nil {
		return []models.Inbox{}, err
	}

	var inboxes []models.Inbox
	if i.isAdminHasAccessToAllInboxes(ctx) {
		inboxes, err = i.InboxRepository.ListInboxes(nil, organizationId, nil)
	} else {
		availableInboxIds, err := i.getAvailableInboxes(ctx)
		if err != nil {
			return []models.Inbox{}, err
		} else if len(availableInboxIds) == 0 {
			return []models.Inbox{}, nil
		}
		inboxes, err = i.InboxRepository.ListInboxes(nil, organizationId, availableInboxIds)
		if err != nil {
			return []models.Inbox{}, err
		}
	}
	if err != nil {
		return []models.Inbox{}, err
	}

	for _, inbox := range inboxes {
		if err = i.EnforceSecurity.ReadInbox(inbox); err != nil {
			return []models.Inbox{}, err
		}
	}

	return inboxes, nil
}

func (i *InboxReader) isAdminHasAccessToAllInboxes(ctx context.Context) bool {
	return i.Credentials.Role == models.ADMIN || i.Credentials.Role == models.MARBLE_ADMIN
}

func (i *InboxReader) getAvailableInboxes(ctx context.Context) ([]string, error) {
	availableInboxIds := make([]string, 0)

	userId := i.Credentials.ActorIdentity.UserId
	inboxUsers, err := i.InboxRepository.ListInboxUsers(nil, models.InboxUserFilterInput{UserId: userId})
	if err != nil {
		return []string{}, err
	}

	for _, inboxUser := range inboxUsers {
		availableInboxIds = append(availableInboxIds, inboxUser.InboxId)
	}
	return availableInboxIds, nil
}
