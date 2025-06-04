package inboxes

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
)

type InboxRepository interface {
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error)
	ListInboxes(ctx context.Context, exec repositories.Executor, organizationId string,
		inboxIds []uuid.UUID, withCaseCount bool) ([]models.Inbox, error)
	ListInboxUsers(ctx context.Context, exec repositories.Executor,
		filters models.InboxUserFilterInput) ([]models.InboxUser, error) // Assuming filters.InboxId and filters.UserId are now UUIDs based on prior changes to models.InboxUserFilterInput
}

type EnforceSecurityInboxes interface {
	ReadInbox(i models.Inbox) error
	ReadInboxMetadata(inbox models.Inbox) error
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

func (i *InboxReader) GetInboxById(ctx context.Context, inboxId uuid.UUID) (models.Inbox, error) {
	inbox, err := i.InboxRepository.GetInboxById(ctx, i.ExecutorFactory.NewExecutor(), inboxId)
	if err != nil {
		return models.Inbox{}, err
	}

	if err = i.EnforceSecurity.ReadInbox(inbox); err != nil {
		return models.Inbox{}, err
	}

	return inbox, err
}

func (i *InboxReader) GetEscalationInboxMetadata(ctx context.Context, inboxId uuid.UUID) (models.InboxMetadata, error) {
	inbox, err := i.InboxRepository.GetInboxById(ctx, i.ExecutorFactory.NewExecutor(), inboxId)
	if err != nil {
		return models.InboxMetadata{}, err
	}

	if err := i.EnforceSecurity.ReadInboxMetadata(inbox); err != nil {
		return models.InboxMetadata{}, err
	}

	return inbox.GetMetadata(), nil
}

func (i *InboxReader) ListInboxes(
	ctx context.Context,
	exec repositories.Executor,
	organizationId string,
	withCaseCount bool,
) ([]models.Inbox, error) {
	var inboxes []models.Inbox
	var err error
	if i.isAdminHasAccessToAllInboxes() {
		inboxes, err = i.InboxRepository.ListInboxes(ctx, exec, organizationId, nil, withCaseCount)
		if err != nil {
			return nil, err
		}
	} else {
		availableInboxIds, err := i.getAvailableInboxes(ctx, exec)
		if err != nil {
			return nil, err
		}
		if len(availableInboxIds) == 0 {
			return []models.Inbox{}, nil
		}
		inboxes, err = i.InboxRepository.ListInboxes(ctx, exec, organizationId, availableInboxIds, withCaseCount)
		if err != nil {
			return nil, err
		}
	}

	for _, inbox := range inboxes {
		if err := i.EnforceSecurity.ReadInbox(inbox); err != nil {
			return nil, err
		}
	}

	return inboxes, nil
}

func (i *InboxReader) isAdminHasAccessToAllInboxes() bool {
	return i.Credentials.Role == models.ADMIN || i.Credentials.Role == models.MARBLE_ADMIN
}

func (i *InboxReader) getAvailableInboxes(ctx context.Context, exec repositories.Executor) ([]uuid.UUID, error) {
	availableInboxIds := make([]uuid.UUID, 0)

	// Assuming i.Credentials.ActorIdentity.UserId is compatible with uuid.UUID for models.InboxUserFilterInput.UserId
	// UserId in InboxUserFilterInput is models.UserId (string), so no parsing needed for i.Credentials.ActorIdentity.UserId.
	userId := i.Credentials.ActorIdentity.UserId

	inboxUsers, err := i.InboxRepository.ListInboxUsers(ctx, exec, models.InboxUserFilterInput{
		UserId: models.UserId(userId), // Pass as models.UserId (string)
	})
	if err != nil {
		return []uuid.UUID{}, err
	}

	for _, inboxUser := range inboxUsers {
		availableInboxIds = append(availableInboxIds, inboxUser.InboxId) // inboxUser.InboxId is already uuid.UUID
	}
	return availableInboxIds, nil
}
