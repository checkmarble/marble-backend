package usecases

import (
	"context"
	"slices"
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
	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	userRepository      repositories.UserRepository
}

func (usecase *UserUseCase) AddUser(ctx context.Context, createUser models.CreateUser) (models.User, error) {
	if !slices.Contains(models.GetValidUserRoles(), createUser.Role) {
		return models.User{}, errors.Wrap(models.BadParameterError, "Invalid role received")
	}

	if err := usecase.enforceUserSecurity.CreateUser(createUser); err != nil {
		return models.User{}, err
	}
	createdUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.User, error) {
			// cleanup spaces
			createUser.Email = strings.TrimSpace(createUser.Email)
			// lowercase email to maintain uniqueness
			createUser.Email = strings.ToLower(createUser.Email)

			createdUserUuid, err := usecase.userRepository.CreateUser(ctx, tx, createUser)
			if repositories.IsUniqueViolationError(err) {
				return models.User{}, models.ConflictError
			}
			if err != nil {
				return models.User{}, err
			}
			return usecase.userRepository.UserById(ctx, tx, createdUserUuid)
		},
	)
	if err != nil {
		return models.User{}, err
	}
	tracking.TrackEvent(context.Background(), models.AnalyticsUserCreated, map[string]interface{}{
		"user_id": createdUser.UserId,
	})

	return createdUser, nil
}

func (usecase *UserUseCase) UpdateUser(ctx context.Context, updateUser models.UpdateUser) (models.User, error) {
	if updateUser.Role != nil && !slices.Contains(models.GetValidUserRoles(), *updateUser.Role) {
		return models.User{}, errors.Wrap(models.BadParameterError, "Invalid role received")
	}

	updatedUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.User, error) {
			user, err := usecase.userRepository.UserById(ctx, tx, updateUser.UserId)
			if err != nil {
				return models.User{}, err
			}
			if err := usecase.enforceUserSecurity.UpdateUser(user, updateUser); err != nil {
				return models.User{}, err
			}
			if err := usecase.userRepository.UpdateUser(ctx, tx, updateUser); err != nil {
				return models.User{}, err
			}
			return usecase.userRepository.UserById(ctx, tx, updateUser.UserId)
		},
	)
	if err != nil {
		return models.User{}, err
	}
	tracking.TrackEvent(ctx, models.AnalyticsUserUpdated, map[string]interface{}{
		"user_id": updatedUser.UserId,
	})

	return updatedUser, nil
}

func (usecase *UserUseCase) DeleteUser(ctx context.Context, userId, currentUserId string) error {
	if userId == currentUserId {
		return errors.Wrap(models.ForbiddenError, "cannot delete yourself")
	}
	exec := usecase.executorFactory.NewExecutor()
	user, err := usecase.userRepository.UserById(ctx, exec, userId)
	if err != nil {
		return err
	}
	if err := usecase.enforceUserSecurity.DeleteUser(user); err != nil {
		return err
	}
	err = usecase.userRepository.DeleteUser(ctx, exec, models.UserId(userId))
	if err != nil {
		return err
	}
	tracking.TrackEvent(context.Background(), models.AnalyticsUserDeleted, map[string]interface{}{
		"user_id": userId,
	})

	return nil
}

func (usecase *UserUseCase) ListUsers(ctx context.Context, organisationIdFilter *string) ([]models.User, error) {
	if err := usecase.enforceUserSecurity.ListUsers(organisationIdFilter); err != nil {
		return nil, err
	}

	exec := usecase.executorFactory.NewExecutor()
	users, err := usecase.userRepository.ListUsers(ctx, exec, organisationIdFilter)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if err = usecase.enforceUserSecurity.ReadUser(u); err != nil {
			return nil, err
		}
	}

	return users, nil
}

func (usecase *UserUseCase) GetUser(ctx context.Context, userID string) (models.User, error) {
	user, err := usecase.userRepository.UserById(ctx, usecase.executorFactory.NewExecutor(), userID)
	if err != nil {
		return models.User{}, err
	}

	if err := usecase.enforceUserSecurity.ReadUser(user); err != nil {
		return models.User{}, err
	}

	return user, nil
}
