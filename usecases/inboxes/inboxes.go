package inboxes

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type InboxRepository interface {
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId string) (models.Inbox, error)
	ListInboxes(ctx context.Context, exec repositories.Executor, organizationId string,
		inboxIds []string, withCaseCount bool) ([]models.Inbox, error)
	ListInboxUsers(ctx context.Context, exec repositories.Executor,
		filters models.InboxUserFilterInput) ([]models.InboxUser, error)
}

type EnforceSecurityInboxes interface {
	ReadInbox(i models.Inbox) error
	CreateInbox(organizationId string) error
	ReadInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error
	CreateInboxUser(i models.CreateInboxUserInput, actorInboxUsers []models.InboxUser,
		targetInbox models.Inbox, targetUser models.User) error
}

type InboxReader struct {
	EnforceSecurity EnforceSecurityInboxes
	InboxRepository InboxRepository
	Credentials     models.Credentials
	ExecutorFactory executor_factory.ExecutorFactory
}

func (i *InboxReader) GetInboxById(ctx context.Context, inboxId string) (models.Inbox, error) {
	inbox, err := i.InboxRepository.GetInboxById(ctx, i.ExecutorFactory.NewExecutor(), inboxId)
	if err != nil {
		return models.Inbox{}, err
	}

	if err = i.EnforceSecurity.ReadInbox(inbox); err != nil {
		return models.Inbox{}, err
	}

	return inbox, err
}

func (i *InboxReader) ListInboxes(
	ctx context.Context,
	exec repositories.Executor,
	organizationId string,
	withCaseCount bool,
) ([]models.Inbox, error) {
	var inboxes []models.Inbox
	var err error
	if i.isAdminHasAccessToAllInboxes(ctx) {
		inboxes, err = i.InboxRepository.ListInboxes(ctx, exec, organizationId, nil, withCaseCount)
	} else {
		availableInboxIds, err := i.getAvailableInboxes(ctx, exec)
		if err != nil {
			return []models.Inbox{}, err
		} else if len(availableInboxIds) == 0 {
			return []models.Inbox{}, nil
		}
		inboxes, err = i.InboxRepository.ListInboxes(ctx, exec, organizationId, availableInboxIds, withCaseCount)
		if err != nil {
			return []models.Inbox{}, err
		}
	}
	if err != nil {
		return []models.Inbox{}, err
	}

	for _, inbox := range inboxes {
		if err := i.EnforceSecurity.ReadInbox(inbox); err != nil {
			return []models.Inbox{}, err
		}
	}

	return inboxes, nil
}

func (i *InboxReader) isAdminHasAccessToAllInboxes(ctx context.Context) bool {
	return i.Credentials.Role == models.ADMIN || i.Credentials.Role == models.MARBLE_ADMIN
}

func (i *InboxReader) getAvailableInboxes(ctx context.Context, exec repositories.Executor) ([]string, error) {
	availableInboxIds := make([]string, 0)

	userId := i.Credentials.ActorIdentity.UserId
	inboxUsers, err := i.InboxRepository.ListInboxUsers(ctx, exec, models.InboxUserFilterInput{
		UserId: userId,
	})
	if err != nil {
		return []string{}, err
	}

	for _, inboxUser := range inboxUsers {
		availableInboxIds = append(availableInboxIds, inboxUser.InboxId)
	}
	return availableInboxIds, nil
}
