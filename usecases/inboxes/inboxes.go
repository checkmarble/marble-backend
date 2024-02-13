package inboxes

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type InboxRepository interface {
	GetInboxById(ctx context.Context, tx repositories.Transaction_deprec, inboxId string) (models.Inbox, error)
	ListInboxes(ctx context.Context, tx repositories.Transaction_deprec, organizationId string, inboxIds []string, withCaseCount bool) ([]models.Inbox, error)
	ListInboxUsers(ctx context.Context, tx repositories.Transaction_deprec, filters models.InboxUserFilterInput) ([]models.InboxUser, error)
}

type EnforceSecurityInboxes interface {
	ReadInbox(i models.Inbox) error
	CreateInbox(organizationId string) error
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
	inbox, err := i.InboxRepository.GetInboxById(ctx, nil, inboxId)
	if err != nil {
		return models.Inbox{}, err
	}

	if err = i.EnforceSecurity.ReadInbox(inbox); err != nil {
		return models.Inbox{}, err
	}

	return inbox, err
}

func (i *InboxReader) ListInboxes(ctx context.Context, tx repositories.Transaction_deprec, withCaseCount bool) ([]models.Inbox, error) {
	organizationId, err := i.OrganizationIdOfContext()
	if err != nil {
		return []models.Inbox{}, err
	}

	var inboxes []models.Inbox
	if i.isAdminHasAccessToAllInboxes(ctx) {
		inboxes, err = i.InboxRepository.ListInboxes(ctx, tx, organizationId, nil, withCaseCount)
	} else {
		availableInboxIds, err := i.getAvailableInboxes(ctx, tx)
		if err != nil {
			return []models.Inbox{}, err
		} else if len(availableInboxIds) == 0 {
			return []models.Inbox{}, nil
		}
		inboxes, err = i.InboxRepository.ListInboxes(ctx, tx, organizationId, availableInboxIds, withCaseCount)
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

func (i *InboxReader) getAvailableInboxes(ctx context.Context, tx repositories.Transaction_deprec) ([]string, error) {
	availableInboxIds := make([]string, 0)

	userId := i.Credentials.ActorIdentity.UserId
	inboxUsers, err := i.InboxRepository.ListInboxUsers(ctx, tx, models.InboxUserFilterInput{UserId: userId})
	if err != nil {
		return []string{}, err
	}

	for _, inboxUser := range inboxUsers {
		availableInboxIds = append(availableInboxIds, inboxUser.InboxId)
	}
	return availableInboxIds, nil
}
