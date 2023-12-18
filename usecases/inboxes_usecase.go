package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/analytics"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type InboxRepository interface {
	GetInboxById(tx repositories.Transaction, inboxId string) (models.Inbox, error)
	ListInboxes(tx repositories.Transaction, organizationId string, inboxIds []string, withCaseCount bool) ([]models.Inbox, error)
	CreateInbox(tx repositories.Transaction, createInboxAttributes models.CreateInboxInput, newInboxId string) error
	UpdateInbox(tx repositories.Transaction, inboxId, name string) error
	SoftDeleteInbox(tx repositories.Transaction, inboxId string) error

	ListOrganizationCases(tx repositories.Transaction, filters models.CaseFilters, pagination models.PaginationAndSorting) ([]models.CaseWithRank, error)
}

type EnforceSecurityInboxes interface {
	ReadInbox(i models.Inbox) error
	CreateInbox(organizationId string) error
}

type InboxUsecase struct {
	transactionFactory      transaction.TransactionFactory
	enforceSecurity         EnforceSecurityInboxes
	organizationIdOfContext func() (string, error)
	inboxRepository         InboxRepository
	userRepository          repositories.UserRepository
	credentials             models.Credentials
	inboxReader             inboxes.InboxReader
	inboxUsers              inboxes.InboxUsers
}

func (usecase *InboxUsecase) GetInboxById(ctx context.Context, inboxId string) (models.Inbox, error) {
	return usecase.inboxReader.GetInboxById(ctx, inboxId)
}

func (usecase *InboxUsecase) ListInboxes(ctx context.Context, withCaseCount bool) ([]models.Inbox, error) {
	return usecase.inboxReader.ListInboxes(ctx, nil, withCaseCount)
}

func (usecase *InboxUsecase) CreateInbox(ctx context.Context, input models.CreateInboxInput) (models.Inbox, error) {
	inbox, err := transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Inbox, error) {
			if err := usecase.enforceSecurity.CreateInbox(input.OrganizationId); err != nil {
				return models.Inbox{}, err
			}

			newInboxId := utils.NewPrimaryKey(input.OrganizationId)
			if err := usecase.inboxRepository.CreateInbox(tx, input, newInboxId); err != nil {
				return models.Inbox{}, err
			}

			inbox, err := usecase.inboxRepository.GetInboxById(tx, newInboxId)
			return inbox, err
		})

	if err != nil {
		return models.Inbox{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsInboxCreated, map[string]interface{}{"inbox_id": inbox.Id})
	return inbox, nil
}

func (usecase *InboxUsecase) UpdateInbox(ctx context.Context, inboxId, name string) (models.Inbox, error) {
	inbox, err := transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Inbox, error) {
			inbox, err := usecase.inboxRepository.GetInboxById(tx, inboxId)
			if err != nil {
				return models.Inbox{}, err
			}

			if inbox.Status != models.InboxStatusActive {
				return models.Inbox{}, errors.Wrap(models.ForbiddenError, "This inbox is archived and cannot be updated")
			}

			if err := usecase.enforceSecurity.CreateInbox(inbox.OrganizationId); err != nil {
				return models.Inbox{}, err
			}

			if err := usecase.inboxRepository.UpdateInbox(tx, inboxId, name); err != nil {
				return models.Inbox{}, err
			}

			return usecase.inboxRepository.GetInboxById(tx, inboxId)
		})

	if err != nil {
		return models.Inbox{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsInboxUpdated, map[string]interface{}{"inbox_id": inbox.Id})
	return inbox, nil
}

func (usecase *InboxUsecase) DeleteInbox(ctx context.Context, inboxId string) error {
	err := usecase.transactionFactory.Transaction(
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) error {
			inbox, err := usecase.inboxRepository.GetInboxById(tx, inboxId)
			if err != nil {
				return err
			}

			if inbox.Status != models.InboxStatusActive {
				return errors.Wrap(models.ForbiddenError, "This inbox is already archived")
			}

			if err := usecase.enforceSecurity.CreateInbox(inbox.OrganizationId); err != nil {
				return err
			}

			cases, err := usecase.inboxRepository.ListOrganizationCases(tx,
				models.CaseFilters{InboxIds: []string{inboxId}, OrganizationId: inbox.OrganizationId},
				models.PaginationAndSorting{Limit: 1, Order: models.SortingOrderDesc, Sorting: models.CasesSortingCreatedAt},
			)
			if err != nil {
				return err
			}
			if len(cases) > 0 {
				return errors.Wrap(models.ForbiddenError, "This inbox is associated with cases and cannot be deleted")
			}

			return usecase.inboxRepository.SoftDeleteInbox(tx, inboxId)
		})

	if err != nil {
		return err
	}

	analytics.TrackEvent(ctx, models.AnalyticsInboxDeleted, map[string]interface{}{"inbox_id": inboxId})
	return nil
}

func (usecase *InboxUsecase) GetInboxUserById(ctx context.Context, inboxUserId string) (models.InboxUser, error) {
	return usecase.inboxUsers.GetInboxUserById(ctx, inboxUserId)
}

func (usecase *InboxUsecase) ListInboxUsers(ctx context.Context, inboxId string) ([]models.InboxUser, error) {
	return usecase.inboxUsers.ListInboxUsers(ctx, inboxId)
}

func (usecase *InboxUsecase) ListAllInboxUsers() ([]models.InboxUser, error) {
	return usecase.inboxUsers.ListAllInboxUsers()
}

func (usecase *InboxUsecase) CreateInboxUser(ctx context.Context, input models.CreateInboxUserInput) (models.InboxUser, error) {
	return usecase.inboxUsers.CreateInboxUser(ctx, input)
}

func (usecase *InboxUsecase) UpdateInboxUser(ctx context.Context, inboxUserId string, role models.InboxUserRole) (models.InboxUser, error) {
	return usecase.inboxUsers.UpdateInboxUser(ctx, inboxUserId, role)
}

func (usecase *InboxUsecase) DeleteInboxUser(ctx context.Context, inboxUserId string) error {
	return usecase.inboxUsers.DeleteInboxUser(ctx, inboxUserId)
}
