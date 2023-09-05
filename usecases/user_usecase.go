package usecases

import (
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type UserUseCase struct {
	enforceAdminSecurity security.EnforceSecurityAdmin
	transactionFactory   repositories.TransactionFactory
	userRepository       repositories.UserRepository
}

func (usecase *UserUseCase) AddUser(createUser models.CreateUser) (models.User, error) {
	if err := usecase.enforceAdminSecurity.CreateUser(); err != nil {
		return models.User{}, err
	}
	return repositories.TransactionReturnValue(
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
}

func (usecase *UserUseCase) DeleteUser(userID string) error {
	if err := usecase.enforceAdminSecurity.DeleteUser(); err != nil {
		return err
	}
	return usecase.transactionFactory.Transaction(
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) error {
			return usecase.userRepository.DeleteUser(tx, models.UserId(userID))
		},
	)
}

func (usecase *UserUseCase) GetAllUsers() ([]models.User, error) {
	if err := usecase.enforceAdminSecurity.ListUser(); err != nil {
		return []models.User{}, err
	}
	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.User, error) {
			return usecase.userRepository.AllUsers(tx)
		},
	)
}

func (usecase *UserUseCase) GetUser(userID string) (models.User, error) {
	if err := usecase.enforceAdminSecurity.ListUser(); err != nil {
		return models.User{}, err
	}
	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.User, error) {
			return usecase.userRepository.UserByID(tx, models.UserId(userID))
		},
	)
}
