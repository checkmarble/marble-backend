package inboxes

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type InboxUserRepository interface {
	GetInboxById(ctx context.Context, exec repositories.Executor, inboxId uuid.UUID) (models.Inbox, error)
	GetInboxUserById(ctx context.Context, exec repositories.Executor, inboxUserId uuid.UUID) (models.InboxUser, error)
	ListInboxUsers(ctx context.Context, exec repositories.Executor,
		filters models.InboxUserFilterInput) ([]models.InboxUser, error)
	CreateInboxUser(ctx context.Context, exec repositories.Executor,
		createInboxUserAttributes models.CreateInboxUserInput, newInboxUserId uuid.UUID) error
	UpdateInboxUser(ctx context.Context, exec repositories.Executor, inboxUserId uuid.UUID, role models.InboxUserRole) error
	DeleteInboxUser(ctx context.Context, exec repositories.Executor, inboxUserId uuid.UUID) error
}

type EnforceSecurityInboxUsers interface {
	ReadInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error
	CreateInboxUser(i models.CreateInboxUserInput, actorInboxUsers []models.InboxUser,
		targetInbox models.Inbox, targetUser models.User) error
	UpdateInboxUser(inboxUser models.InboxUser, actorInboxUsers []models.InboxUser) error
}

type InboxUsers struct {
	TransactionFactory  executor_factory.TransactionFactory
	ExecutorFactory     executor_factory.ExecutorFactory
	EnforceSecurity     EnforceSecurityInboxUsers
	InboxUserRepository InboxUserRepository
	UserRepository      repositories.UserRepository
	Credentials         models.Credentials
}

func (usecase *InboxUsers) GetInboxUserById(ctx context.Context, inboxUserId uuid.UUID) (models.InboxUser, error) {
	exec := usecase.ExecutorFactory.NewExecutor()
	inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(ctx, exec, inboxUserId)
	if err != nil {
		return models.InboxUser{}, err
	}

	actorUserId := usecase.Credentials.ActorIdentity.UserId
	thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, exec, models.InboxUserFilterInput{
		UserId: actorUserId,
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

func (usecase *InboxUsers) ListInboxUsers(ctx context.Context, inboxId uuid.UUID) ([]models.InboxUser, error) {
	exec := usecase.ExecutorFactory.NewExecutor()
	actorUserId := usecase.Credentials.ActorIdentity.UserId
	thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, exec, models.InboxUserFilterInput{
		UserId: actorUserId,
	})
	if err != nil {
		return []models.InboxUser{}, err
	}

	inboxUsers, err := usecase.InboxUserRepository.ListInboxUsers(ctx, exec, models.InboxUserFilterInput{
		InboxId: inboxId, // inboxId is already uuid.UUID
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

	return usecase.InboxUserRepository.ListInboxUsers(ctx,
		usecase.ExecutorFactory.NewExecutor(), models.InboxUserFilterInput{})
}

func (usecase *InboxUsers) CreateInboxUser(ctx context.Context, input models.CreateInboxUserInput) (models.InboxUser, error) {
	inboxUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.TransactionFactory,
		func(tx repositories.Transaction) (models.InboxUser, error) {
			actorUserId := usecase.Credentials.ActorIdentity.UserId
			thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, tx, models.InboxUserFilterInput{
				UserId: actorUserId,
			})
			if err != nil {
				return models.InboxUser{}, err
			}

			targetUser, err := usecase.UserRepository.UserById(ctx, tx, input.UserId.String())
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

			newInboxUserUUID := uuid.New()
			if err := usecase.InboxUserRepository.CreateInboxUser(ctx, tx, input, newInboxUserUUID); err != nil {
				if repositories.IsUniqueViolationError(err) {
					return models.InboxUser{}, errors.Wrap(models.ConflictError,
						"This combination of user_id and inbox_user_id already exists")
				}
				return models.InboxUser{}, err
			}

			inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(ctx, tx, newInboxUserUUID)

			return inboxUser, err
		})
	if err != nil {
		return models.InboxUser{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsInboxUserCreated, map[string]interface{}{
		"inbox_user_id": inboxUser.Id,
	})
	return inboxUser, nil
}

func (usecase *InboxUsers) UpdateInboxUser(ctx context.Context, inboxUserId uuid.UUID, role models.InboxUserRole) (models.InboxUser, error) {
	inboxUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.TransactionFactory,
		func(tx repositories.Transaction) (models.InboxUser, error) {
			actorUserId := usecase.Credentials.ActorIdentity.UserId
			thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, tx, models.InboxUserFilterInput{
				UserId: actorUserId,
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

	tracking.TrackEvent(ctx, models.AnalyticsInboxUserUpdated, map[string]interface{}{
		"inbox_user_id": inboxUser.Id,
	})
	return inboxUser, nil
}

func (usecase *InboxUsers) DeleteInboxUser(ctx context.Context, inboxUserId uuid.UUID) error {
	err := usecase.TransactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			inboxUser, err := usecase.InboxUserRepository.GetInboxUserById(ctx, tx, inboxUserId)
			if err != nil {
				return err
			}

			actorUserId := usecase.Credentials.ActorIdentity.UserId
			thisUsersInboxes, err := usecase.InboxUserRepository.ListInboxUsers(ctx, tx, models.InboxUserFilterInput{
				UserId: actorUserId,
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

	tracking.TrackEvent(ctx, models.AnalyticsInboxUserDeleted, map[string]interface{}{
		"inbox_user_id": inboxUserId,
	})
	return nil
}
