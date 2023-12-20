package inboxes

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/analytics"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

type InboxUserRepository interface {
	GetInboxById(tx repositories.Transaction, inboxId string) (models.Inbox, error)
	GetInboxUserById(tx repositories.Transaction, inboxUserId string) (models.InboxUser, error)
	ListInboxUsers(tx repositories.Transaction, filters models.InboxUserFilterInput) ([]models.InboxUser, error)
	CreateInboxUser(tx repositories.Transaction, createInboxUserAttributes models.CreateInboxUserInput, newInboxUserId string) error
	UpdateInboxUser(tx repositories.Transaction, inboxUserId string, role models.InboxUserRole) error
	DeleteInboxUser(tx repositories.Transaction, inboxUserId string) error
}

type EnforceSecurityInboxUsers interface {
	ReadInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error
	CreateInboxUser(i models.CreateInboxUserInput, actorInboxUsers []models.InboxUser, targetInbox models.Inbox, targetUser models.User) error
	UpdateInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error
}

type InboxUsers struct {
	TransactionFactory  transaction.TransactionFactory
	EnforceSecurity     EnforceSecurityInboxUsers
	InboxUserRepository InboxUserRepository
	UserRepository      repositories.UserRepository
	Credentials         models.Credentials
}

func (usecase *InboxUsers) GetInboxUserById(ctx context.Context, inboxUserId string) (models.InboxUser, error) {
	inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(nil, inboxUserId)
	if err != nil {
		return models.InboxUser{}, err
	}

	thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(nil, models.InboxUserFilterInput{
		UserId: usecase.Credentials.ActorIdentity.UserId,
	})
	if err != nil {
		return models.InboxUser{}, err
	}

	err = usecase.EnforceSecurity.ReadInboxUser(inboxUser, thisUsersInboxes)
	if err != nil {
		return models.InboxUser{}, err
	}

	return inboxUser, nil
}

func (usecase *InboxUsers) ListInboxUsers(ctx context.Context, inboxId string) ([]models.InboxUser, error) {
	thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(nil, models.InboxUserFilterInput{
		UserId: usecase.Credentials.ActorIdentity.UserId,
	})
	if err != nil {
		return []models.InboxUser{}, err
	}

	inboxUsers, err := usecase.InboxUserRepository.ListInboxUsers(nil, models.InboxUserFilterInput{
		InboxId: inboxId,
	})
	if err != nil {
		return []models.InboxUser{}, err
	}

	for _, inboxUser := range inboxUsers {
		err = usecase.EnforceSecurity.ReadInboxUser(inboxUser, thisUsersInboxes)
		if err != nil {
			return []models.InboxUser{}, err
		}
	}
	return inboxUsers, nil
}

func (usecase *InboxUsers) ListAllInboxUsers() ([]models.InboxUser, error) {
	if usecase.Credentials.Role != models.ADMIN && usecase.Credentials.Role != models.MARBLE_ADMIN {
		return []models.InboxUser{}, errors.New("only admins can list all inbox users")
	}

	return usecase.InboxUserRepository.ListInboxUsers(nil, models.InboxUserFilterInput{})
}

func (usecase *InboxUsers) CreateInboxUser(ctx context.Context, input models.CreateInboxUserInput) (models.InboxUser, error) {
	inboxUser, err := transaction.TransactionReturnValue(
		usecase.TransactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.InboxUser, error) {
			thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(tx, models.InboxUserFilterInput{
				UserId: usecase.Credentials.ActorIdentity.UserId,
			})
			if err != nil {
				return models.InboxUser{}, err
			}

			targetUser, err := usecase.UserRepository.UserByID(tx, models.UserId(input.UserId))
			if err != nil {
				return models.InboxUser{}, err
			}
			targetInbox, err := usecase.InboxUserRepository.GetInboxById(tx, input.InboxId)
			if err != nil {
				return models.InboxUser{}, err
			}

			err = usecase.EnforceSecurity.CreateInboxUser(input, thisUsersInboxes, targetInbox, targetUser)
			if err != nil {
				return models.InboxUser{}, err
			}

			newInboxUserId := utils.NewPrimaryKey(input.InboxId)
			if err := usecase.InboxUserRepository.CreateInboxUser(tx, input, newInboxUserId); err != nil {
				if repositories.IsUniqueViolationError(err) {
					return models.InboxUser{}, errors.Wrap(models.DuplicateValueError, "This combination of user_id and inbox_user_id already exists")
				}
				return models.InboxUser{}, err
			}

			inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(tx, newInboxUserId)

			return inboxUser, err
		})

	if err != nil {
		return models.InboxUser{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsInboxUserCreated, map[string]interface{}{"inbox_user_id": inboxUser.Id})
	return inboxUser, nil
}

func (usecase *InboxUsers) UpdateInboxUser(ctx context.Context, inboxUserId string, role models.InboxUserRole) (models.InboxUser, error) {
	inboxUser, err := transaction.TransactionReturnValue(
		usecase.TransactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.InboxUser, error) {
			thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(tx, models.InboxUserFilterInput{
				UserId: usecase.Credentials.ActorIdentity.UserId,
			})
			if err != nil {
				return models.InboxUser{}, err
			}

			inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(tx, inboxUserId)
			if err != nil {
				return models.InboxUser{}, err
			}

			err = usecase.EnforceSecurity.UpdateInboxUser(inboxUser, thisUsersInboxes)
			if err != nil {
				return models.InboxUser{}, err
			}

			if err := usecase.InboxUserRepository.UpdateInboxUser(tx, inboxUserId, role); err != nil {
				return models.InboxUser{}, err
			}

			return usecase.InboxUserRepository.GetInboxUserById(tx, inboxUserId)
		})

	if err != nil {
		return models.InboxUser{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsInboxUserUpdated, map[string]interface{}{"inbox_user_id": inboxUser.Id})
	return inboxUser, nil
}

func (usecase *InboxUsers) DeleteInboxUser(ctx context.Context, inboxUserId string) error {
	err := usecase.TransactionFactory.Transaction(
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) error {
			inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(tx, inboxUserId)
			if err != nil {
				return err
			}

			thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(tx, models.InboxUserFilterInput{
				UserId: usecase.Credentials.ActorIdentity.UserId,
			})
			if err != nil {
				return err
			}

			err = usecase.EnforceSecurity.UpdateInboxUser(inboxUser, thisUsersInboxes)
			if err != nil {
				return err
			}

			return usecase.InboxUserRepository.DeleteInboxUser(tx, inboxUserId)
		})

	if err != nil {
		return err
	}

	analytics.TrackEvent(ctx, models.AnalyticsInboxUserDeleted, map[string]interface{}{"inbox_user_id": inboxUserId})
	return nil
}
