package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
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

type InboxUsecase struct {
	transactionFactory      transaction.TransactionFactory
	enforceSecurity         EnforceSecurityInboxes
	organizationIdOfContext func() (string, error)
	inboxRepository         InboxRepository
	userRepository          repositories.UserRepository
	credentials             models.Credentials
	inboxReader             inboxes.InboxReader
}

func (usecase *InboxUsecase) GetInboxById(ctx context.Context, inboxId string) (models.Inbox, error) {
	return usecase.inboxReader.GetInboxById(ctx, inboxId)
}

func (usecase *InboxUsecase) ListInboxes(ctx context.Context) ([]models.Inbox, error) {
	return usecase.inboxReader.ListInboxes(ctx)
}

func (usecase *InboxUsecase) CreateInbox(ctx context.Context, input models.CreateInboxInput) (models.Inbox, error) {
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Inbox, error) {
			if err := usecase.enforceSecurity.CreateInbox(input); err != nil {
				return models.Inbox{}, err
			}

			newInboxId := utils.NewPrimaryKey(input.OrganizationId)
			if err := usecase.inboxRepository.CreateInbox(tx, input, newInboxId); err != nil {
				return models.Inbox{}, err
			}

			inbox, err := usecase.inboxRepository.GetInboxById(tx, newInboxId)
			return inbox, err
		})
}

func (usecase *InboxUsecase) GetInboxUserById(ctx context.Context, inboxUserId string) (models.InboxUser, error) {
	inboxUser, err := usecase.inboxRepository.GetInboxUserById(nil, inboxUserId)
	if err != nil {
		return models.InboxUser{}, err
	}

	thisUsersInboxes, err := usecase.inboxRepository.ListInboxUsers(nil, models.InboxUserFilterInput{
		UserId: usecase.credentials.ActorIdentity.UserId,
	})
	if err != nil {
		return models.InboxUser{}, err
	}

	err = usecase.enforceSecurity.ReadInboxUser(inboxUser, thisUsersInboxes)
	if err != nil {
		return models.InboxUser{}, err
	}

	return inboxUser, nil
}

func (usecase *InboxUsecase) ListInboxUsers(ctx context.Context, inboxId string) ([]models.InboxUser, error) {
	thisUsersInboxes, err := usecase.inboxRepository.ListInboxUsers(nil, models.InboxUserFilterInput{
		UserId: usecase.credentials.ActorIdentity.UserId,
	})
	if err != nil {
		return []models.InboxUser{}, err
	}

	inboxUsers, err := usecase.inboxRepository.ListInboxUsers(nil, models.InboxUserFilterInput{
		InboxId: inboxId,
	})
	if err != nil {
		return []models.InboxUser{}, err
	}

	for _, inboxUser := range inboxUsers {
		err = usecase.enforceSecurity.ReadInboxUser(inboxUser, thisUsersInboxes)
		if err != nil {
			return []models.InboxUser{}, err
		}
	}
	return inboxUsers, nil
}

func (usecase *InboxUsecase) CreateInboxUser(ctx context.Context, input models.CreateInboxUserInput) (models.InboxUser, error) {
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.InboxUser, error) {
			thisUsersInboxes, err := usecase.inboxRepository.ListInboxUsers(tx, models.InboxUserFilterInput{
				UserId: usecase.credentials.ActorIdentity.UserId,
			})
			if err != nil {
				return models.InboxUser{}, err
			}

			targetUser, err := usecase.userRepository.UserByID(tx, models.UserId(input.UserId))
			if err != nil {
				return models.InboxUser{}, err
			}
			targetInbox, err := usecase.inboxRepository.GetInboxById(tx, input.InboxId)
			if err != nil {
				return models.InboxUser{}, err
			}

			err = usecase.enforceSecurity.CreateInboxUser(input, thisUsersInboxes, targetInbox, targetUser)
			if err != nil {
				return models.InboxUser{}, err
			}

			newInboxUserId := utils.NewPrimaryKey(input.InboxId)
			if err := usecase.inboxRepository.CreateInboxUser(tx, input, newInboxUserId); err != nil {
				if repositories.IsUniqueViolationError(err) {
					return models.InboxUser{}, errors.Wrap(models.DuplicateValueError, "This combination of user_id and inbox_user_id already exists")
				}
				return models.InboxUser{}, err
			}

			inboxUser, err := usecase.inboxRepository.GetInboxUserById(tx, newInboxUserId)

			return inboxUser, err
		})
}
