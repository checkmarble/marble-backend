package usecases

import (
	"context"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/analytics"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/cockroachdb/errors"
)

type UserUseCase struct {
	enforceUserSecurity security.EnforceSecurityUser
	transactionFactory  transaction.TransactionFactory
	userRepository      repositories.UserRepository
}

func (usecase *UserUseCase) AddUser(createUser models.CreateUser) (models.User, error) {
	if err := usecase.enforceUserSecurity.CreateUser(createUser.OrganizationId); err != nil {
		return models.User{}, err
	}
	createdUser, err := transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.User, error) {

			// cleanup spaces
			createUser.Email = strings.TrimSpace(createUser.Email)
			// lowercase email to maintain uniqueness
			createUser.Email = strings.ToLower(createUser.Email)

			createdUserUuid, err := usecase.userRepository.CreateUser(tx, createUser)
			if err != nil {
				return models.User{}, err
			}
			return usecase.userRepository.UserByID(tx, createdUserUuid)
		},
	)
	if err != nil {
		return models.User{}, err
	}
	analytics.TrackEvent(context.Background(), models.AnalyticsUserCreated, map[string]interface{}{"user_id": createdUser.UserId})

	return createdUser, nil
}

func (usecase *UserUseCase) UpdateUser(ctx context.Context, updateUser models.UpdateUser) (models.User, error) {
	updatedUser, err := transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.User, error) {
			user, err := usecase.userRepository.UserByID(tx, updateUser.UserId)
			if err != nil {
				return models.User{}, err
			}
			if err := usecase.enforceUserSecurity.UpdateUser(user); err != nil {
				return models.User{}, err
			}
			if err := usecase.userRepository.UpdateUser(tx, updateUser); err != nil {
				return models.User{}, err
			}
			return usecase.userRepository.UserByID(tx, updateUser.UserId)
		},
	)

	if err != nil {
		return models.User{}, err
	}
	analytics.TrackEvent(ctx, models.AnalyticsUserUpdated, map[string]interface{}{"user_id": updatedUser.UserId})

	return updatedUser, nil
}

func (usecase *UserUseCase) DeleteUser(userId, currentUserId string) error {
	if userId == currentUserId {
		return errors.Wrap(models.ForbiddenError, "cannot delete yourself")
	}
	err := usecase.transactionFactory.Transaction(
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) error {
			user, err := usecase.userRepository.UserByID(tx, models.UserId(userId))
			if err != nil {
				return err
			}
			if err := usecase.enforceUserSecurity.DeleteUser(user); err != nil {
				return err
			}
			return usecase.userRepository.DeleteUser(tx, models.UserId(userId))
		},
	)
	if err != nil {
		return err
	}
	analytics.TrackEvent(context.Background(), models.AnalyticsUserDeleted, map[string]interface{}{"user_id": userId})

	return nil
}

func (usecase *UserUseCase) GetAllUsers() ([]models.User, error) {
	if err := usecase.enforceUserSecurity.ListUser(); err != nil {
		return []models.User{}, err
	}
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.User, error) {
			return usecase.userRepository.AllUsers(tx)
		},
	)
}

func (usecase *UserUseCase) GetUser(userID string) (models.User, error) {
	if err := usecase.enforceUserSecurity.ListUser(); err != nil {
		return models.User{}, err
	}
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.User, error) {
			return usecase.userRepository.UserByID(tx, models.UserId(userID))
		},
	)
}
