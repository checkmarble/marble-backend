package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type InboxRepository interface {
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error)
	ListInboxes(ctx context.Context, exec repositories.Executor, organizationId string,
		inboxIds []uuid.UUID, withCaseCount bool) ([]models.Inbox, error)
	CreateInbox(ctx context.Context, exec repositories.Executor,
		createInboxAttributes models.CreateInboxInput, newInboxId uuid.UUID) error
	UpdateInbox(ctx context.Context, exec repositories.Executor,
		inboxId uuid.UUID, name *string, escalationInboxId *uuid.UUID, autoAssignEnabled *bool) error
	SoftDeleteInbox(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) error

	ListOrganizationCases(ctx context.Context, exec repositories.Executor, filters models.CaseFilters,
		pagination models.PaginationAndSorting) ([]models.CaseWithRank, error)
	// Assuming CaseFilters.InboxIds will be updated to []uuid.UUID if necessary
}

type EnforceSecurityInboxes interface {
	ReadInbox(i models.Inbox) error
	ReadInboxMetadata(i models.Inbox) error
	CreateInbox(organizationId string) error
	UpdateInbox(inbox models.Inbox) error
}

type InboxUsecase struct {
	transactionFactory executor_factory.TransactionFactory
	executorFactory    executor_factory.ExecutorFactory
	enforceSecurity    EnforceSecurityInboxes
	inboxRepository    InboxRepository
	userRepository     repositories.UserRepository
	credentials        models.Credentials
	inboxReader        inboxes.InboxReader
	inboxUsers         inboxes.InboxUsers
}

func (usecase *InboxUsecase) GetInboxMetadataById(ctx context.Context, inboxId uuid.UUID) (models.InboxMetadata, error) {
	inbox, err := usecase.inboxRepository.GetInboxById(ctx,
		usecase.executorFactory.NewExecutor(), inboxId)
	if err != nil {
		return models.InboxMetadata{}, errors.Wrap(err, "could not get inbox")
	}

	if err := usecase.enforceSecurity.ReadInboxMetadata(inbox); err != nil {
		return models.InboxMetadata{}, err
	}

	return inbox.GetMetadata(), nil
}

func (usecase *InboxUsecase) GetInboxById(ctx context.Context, inboxId uuid.UUID) (models.Inbox, error) {
	return usecase.inboxReader.GetInboxById(ctx, inboxId)
}

func (usecase *InboxUsecase) ListInboxes(ctx context.Context, organizationId string, withCaseCount bool) ([]models.Inbox, error) {
	return usecase.inboxReader.ListInboxes(ctx, usecase.executorFactory.NewExecutor(), organizationId, withCaseCount)
}

func (usecase *InboxUsecase) ListInboxesMetadata(ctx context.Context, organizationId string) ([]models.InboxMetadata, error) {
	inboxes, err := usecase.inboxRepository.ListInboxes(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, nil, false)
	if err != nil {
		return nil, errors.Wrap(err, "could not list inboxes")
	}

	for _, inbox := range inboxes {
		if err := usecase.enforceSecurity.ReadInboxMetadata(inbox); err != nil {
			return nil, err
		}
	}

	return pure_utils.Map(inboxes, func(i models.Inbox) models.InboxMetadata { return i.GetMetadata() }), nil
}

func (usecase *InboxUsecase) CreateInbox(ctx context.Context, input models.CreateInboxInput) (models.Inbox, error) {
	inbox, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Inbox, error) {
			if err := usecase.enforceSecurity.CreateInbox(input.OrganizationId); err != nil {
				return models.Inbox{}, err
			}

			newInboxIdStr := pure_utils.NewPrimaryKey(input.OrganizationId)
			newInboxUUID, err := uuid.Parse(newInboxIdStr)
			if err != nil {
				return models.Inbox{}, errors.Wrap(err, "failed to parse new inbox ID")
			}
			if err := usecase.inboxRepository.CreateInbox(ctx, tx, input, newInboxUUID); err != nil {
				return models.Inbox{}, err
			}

			inbox, err := usecase.inboxRepository.GetInboxById(ctx, tx, newInboxUUID)
			return inbox, err
		})
	if err != nil {
		return models.Inbox{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsInboxCreated, map[string]interface{}{
		"inbox_id": inbox.Id,
	})
	return inbox, nil
}

func (usecase *InboxUsecase) UpdateInbox(ctx context.Context, inboxId uuid.UUID, name *string, escalationInboxId *uuid.UUID, autoAssignEnabled *bool) (models.Inbox, error) {
	inbox, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Inbox, error) {
			inbox, err := usecase.inboxRepository.GetInboxById(ctx, tx, inboxId)
			if err != nil {
				return models.Inbox{}, err
			}

			if inbox.Status != models.InboxStatusActive {
				return models.Inbox{}, errors.Wrap(models.ForbiddenError,
					"This inbox is archived and cannot be updated")
			}

			if err := usecase.enforceSecurity.UpdateInbox(inbox); err != nil {
				return models.Inbox{}, err
			}

			if err := usecase.inboxRepository.UpdateInbox(ctx, tx, inboxId, name, escalationInboxId, autoAssignEnabled); err != nil {
				return models.Inbox{}, err
			}

			return usecase.inboxRepository.GetInboxById(ctx, tx, inboxId)
		})
	if err != nil {
		return models.Inbox{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsInboxUpdated, map[string]interface{}{
		"inbox_id": inbox.Id,
	})
	return inbox, nil
}

func (usecase *InboxUsecase) DeleteInbox(ctx context.Context, inboxId uuid.UUID) error {
	err := usecase.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			inbox, err := usecase.inboxRepository.GetInboxById(ctx, tx, inboxId)
			if err != nil {
				return err
			}

			if inbox.Status != models.InboxStatusActive {
				return errors.Wrap(models.ForbiddenError, "This inbox is already archived")
			}

			if err := usecase.enforceSecurity.CreateInbox(inbox.OrganizationId); err != nil {
				return err
			}

			cases, err := usecase.inboxRepository.ListOrganizationCases(ctx, tx,
				models.CaseFilters{InboxIds: []uuid.UUID{inboxId}, OrganizationId: inbox.OrganizationId},
				models.PaginationAndSorting{Limit: 1, Order: models.SortingOrderDesc, Sorting: models.CasesSortingCreatedAt},
			)
			if err != nil {
				return err
			}
			if len(cases) > 0 {
				return errors.Wrap(models.ForbiddenError,
					"This inbox is associated with cases and cannot be deleted")
			}

			return usecase.inboxRepository.SoftDeleteInbox(ctx, tx, inboxId)
		})
	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsInboxDeleted, map[string]interface{}{
		"inbox_id": inboxId,
	})
	return nil
}

func (usecase *InboxUsecase) GetInboxUserById(ctx context.Context, inboxUserId uuid.UUID) (models.InboxUser, error) {
	return usecase.inboxUsers.GetInboxUserById(ctx, inboxUserId)
}

func (usecase *InboxUsecase) ListInboxUsers(ctx context.Context, inboxId uuid.UUID) ([]models.InboxUser, error) {
	return usecase.inboxUsers.ListInboxUsers(ctx, inboxId)
}

func (usecase *InboxUsecase) ListAllInboxUsers(ctx context.Context) ([]models.InboxUser, error) {
	return usecase.inboxUsers.ListAllInboxUsers(ctx)
}

func (usecase *InboxUsecase) CreateInboxUser(ctx context.Context, input models.CreateInboxUserInput) (models.InboxUser, error) {
	return usecase.inboxUsers.CreateInboxUser(ctx, input) // input already uses uuid.UUID for IDs from model changes
}

func (usecase *InboxUsecase) UpdateInboxUser(ctx context.Context, inboxUserId uuid.UUID, role *models.InboxUserRole, autoAssignable *bool) (models.InboxUser, error) {
	return usecase.inboxUsers.UpdateInboxUser(ctx, inboxUserId, role, autoAssignable)
}

func (usecase *InboxUsecase) DeleteInboxUser(ctx context.Context, inboxUserId uuid.UUID) error {
	return usecase.inboxUsers.DeleteInboxUser(ctx, inboxUserId)
}
