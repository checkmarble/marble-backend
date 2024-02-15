package usecases

import (
	"context"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"

	"github.com/cockroachdb/errors"
)

type UserUseCase struct {
	enforceUserSecurity security.EnforceSecurityUser
	transactionFactory  executor_factory.TransactionFactory
	userRepository      repositories.UserRepository
}

func (usecase *UserUseCase) AddUser(ctx context.Context, createUser models.CreateUser) (models.User, error) {
	if err := usecase.enforceUserSecurity.CreateUser(createUser.OrganizationId); err != nil {
		return models.User{}, err
	}
	createdUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.User, error) {
			// cleanup spaces
			createUser.Email = strings.TrimSpace(createUser.Email)
			// lowercase email to maintain uniqueness
			createUser.Email = strings.ToLower(createUser.Email)

			createdUserUuid, err := usecase.userRepository.CreateUser(ctx, tx, createUser)
			if repositories.IsUniqueViolationError(err) {
				return models.User{}, models.DuplicateValueError
			}
			if err != nil {
				return models.User{}, err
			}
			return usecase.userRepository.UserByID(ctx, tx, createdUserUuid)
		},
	)
	if err != nil {
		return models.User{}, err
	}
	tracking.TrackEvent(context.Background(), models.AnalyticsUserCreated, map[string]interface{}{"user_id": createdUser.UserId})

	return createdUser, nil
}

func (usecase *UserUseCase) UpdateUser(ctx context.Context, updateUser models.UpdateUser) (models.User, error) {
	updatedUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.User, error) {
			user, err := usecase.userRepository.UserByID(ctx, tx, updateUser.UserId)
			if err != nil {
				return models.User{}, err
			}
			if err := usecase.enforceUserSecurity.UpdateUser(user); err != nil {
				return models.User{}, err
			}
			if err := usecase.userRepository.UpdateUser(ctx, tx, updateUser); err != nil {
				return models.User{}, err
			}
			return usecase.userRepository.UserByID(ctx, tx, updateUser.UserId)
		},
	)

	if err != nil {
		return models.User{}, err
	}
	tracking.TrackEvent(ctx, models.AnalyticsUserUpdated, map[string]interface{}{"user_id": updatedUser.UserId})

	return updatedUser, nil
}

func (usecase *UserUseCase) DeleteUser(ctx context.Context, userId, currentUserId string) error {
	if userId == currentUserId {
		return errors.Wrap(models.ForbiddenError, "cannot delete yourself")
	}
	err := usecase.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Executor) error {
			user, err := usecase.userRepository.UserByID(ctx, tx, models.UserId(userId))
			if err != nil {
				return err
			}
			if err := usecase.enforceUserSecurity.DeleteUser(user); err != nil {
				return err
			}
			return usecase.userRepository.DeleteUser(ctx, tx, models.UserId(userId))
		},
	)
	if err != nil {
		return err
	}
	tracking.TrackEvent(context.Background(), models.AnalyticsUserDeleted, map[string]interface{}{"user_id": userId})

	return nil
}

func (usecase *UserUseCase) GetAllUsers(ctx context.Context) ([]models.User, error) {
	if err := usecase.enforceUserSecurity.ListUser(); err != nil {
		return []models.User{}, err
	}
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) ([]models.User, error) {
			return usecase.userRepository.AllUsers(ctx, tx)
		},
	)
}

func (usecase *UserUseCase) GetUser(ctx context.Context, userID string) (models.User, error) {
	if err := usecase.enforceUserSecurity.ListUser(); err != nil {
		return models.User{}, err
	}
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.User, error) {
			return usecase.userRepository.UserByID(ctx, tx, models.UserId(userID))
		},
	)
}
