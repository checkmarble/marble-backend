package inboxes

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

type InboxUserRepository interface {
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId string) (models.Inbox, error)
	GetInboxUserById(ctx context.Context, exec repositories.Executor, inboxUserId string) (models.InboxUser, error)
	ListInboxUsers(ctx context.Context, exec repositories.Executor, filters models.InboxUserFilterInput) ([]models.InboxUser, error)
	CreateInboxUser(ctx context.Context, exec repositories.Executor, createInboxUserAttributes models.CreateInboxUserInput, newInboxUserId string) error
	UpdateInboxUser(ctx context.Context, exec repositories.Executor, inboxUserId string, role models.InboxUserRole) error
	DeleteInboxUser(ctx context.Context, exec repositories.Executor, inboxUserId string) error
}

type EnforceSecurityInboxUsers interface {
	ReadInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error
	CreateInboxUser(i models.CreateInboxUserInput, actorInboxUsers []models.InboxUser, targetInbox models.Inbox, targetUser models.User) error
	UpdateInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error
}

type InboxUsers struct {
	TransactionFactory  executor_factory.TransactionFactory
	EnforceSecurity     EnforceSecurityInboxUsers
	InboxUserRepository InboxUserRepository
	UserRepository      repositories.UserRepository
	Credentials         models.Credentials
}

func (usecase *InboxUsers) GetInboxUserById(ctx context.Context, inboxUserId string) (models.InboxUser, error) {
	inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(ctx, nil, inboxUserId)
	if err != nil {
		return models.InboxUser{}, err
	}

	thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, nil, models.InboxUserFilterInput{
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
	thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, nil, models.InboxUserFilterInput{
		UserId: usecase.Credentials.ActorIdentity.UserId,
	})
	if err != nil {
		return []models.InboxUser{}, err
	}

	inboxUsers, err := usecase.InboxUserRepository.ListInboxUsers(ctx, nil, models.InboxUserFilterInput{
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

func (usecase *InboxUsers) ListAllInboxUsers(ctx context.Context) ([]models.InboxUser, error) {
	if usecase.Credentials.Role != models.ADMIN && usecase.Credentials.Role != models.MARBLE_ADMIN {
		return []models.InboxUser{}, errors.New("only admins can list all inbox users")
	}

	return usecase.InboxUserRepository.ListInboxUsers(ctx, nil, models.InboxUserFilterInput{})
}

func (usecase *InboxUsers) CreateInboxUser(ctx context.Context, input models.CreateInboxUserInput) (models.InboxUser, error) {
	inboxUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.TransactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Executor) (models.InboxUser, error) {
			thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, tx, models.InboxUserFilterInput{
				UserId: usecase.Credentials.ActorIdentity.UserId,
			})
			if err != nil {
				return models.InboxUser{}, err
			}

			targetUser, err := usecase.UserRepository.UserByID(ctx, tx, models.UserId(input.UserId))
			if err != nil {
				return models.InboxUser{}, err
			}
			targetInbox, err := usecase.InboxUserRepository.GetInboxById(ctx, tx, input.InboxId)
			if err != nil {
				return models.InboxUser{}, err
			}

			err = usecase.EnforceSecurity.CreateInboxUser(input, thisUsersInboxes, targetInbox, targetUser)
			if err != nil {
				return models.InboxUser{}, err
			}

			newInboxUserId := utils.NewPrimaryKey(input.InboxId)
			if err := usecase.InboxUserRepository.CreateInboxUser(ctx, tx, input, newInboxUserId); err != nil {
				if repositories.IsUniqueViolationError(err) {
					return models.InboxUser{}, errors.Wrap(models.DuplicateValueError, "This combination of user_id and inbox_user_id already exists")
				}
				return models.InboxUser{}, err
			}

			inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(ctx, tx, newInboxUserId)

			return inboxUser, err
		})

	if err != nil {
		return models.InboxUser{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsInboxUserCreated, map[string]interface{}{"inbox_user_id": inboxUser.Id})
	return inboxUser, nil
}

func (usecase *InboxUsers) UpdateInboxUser(ctx context.Context, inboxUserId string, role models.InboxUserRole) (models.InboxUser, error) {
	inboxUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.TransactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Executor) (models.InboxUser, error) {
			thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, tx, models.InboxUserFilterInput{
				UserId: usecase.Credentials.ActorIdentity.UserId,
			})
			if err != nil {
				return models.InboxUser{}, err
			}

			inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(ctx, tx, inboxUserId)
			if err != nil {
				return models.InboxUser{}, err
			}

			err = usecase.EnforceSecurity.UpdateInboxUser(inboxUser, thisUsersInboxes)
			if err != nil {
				return models.InboxUser{}, err
			}

			if err := usecase.InboxUserRepository.UpdateInboxUser(ctx, tx, inboxUserId, role); err != nil {
				return models.InboxUser{}, err
			}

			return usecase.InboxUserRepository.GetInboxUserById(ctx, tx, inboxUserId)
		})

	if err != nil {
		return models.InboxUser{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsInboxUserUpdated, map[string]interface{}{"inbox_user_id": inboxUser.Id})
	return inboxUser, nil
}

func (usecase *InboxUsers) DeleteInboxUser(ctx context.Context, inboxUserId string) error {
	err := usecase.TransactionFactory.Transaction(
		ctx,
		func(tx repositories.Executor) error {
			inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(ctx, tx, inboxUserId)
			if err != nil {
				return err
			}

			thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, tx, models.InboxUserFilterInput{
				UserId: usecase.Credentials.ActorIdentity.UserId,
			})
			if err != nil {
				return err
			}

			err = usecase.EnforceSecurity.UpdateInboxUser(inboxUser, thisUsersInboxes)
			if err != nil {
				return err
			}

			return usecase.InboxUserRepository.DeleteInboxUser(ctx, tx, inboxUserId)
		})

	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsInboxUserDeleted, map[string]interface{}{"inbox_user_id": inboxUserId})
	return nil
}
