package usecases

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"strings"
)

type UserUseCase struct {
	transactionFactory repositories.TransactionFactory
	userRepository     repositories.UserRepository
}

func (usecase *UserUseCase) AddUser(createUser models.CreateUser) (models.User, error) {

	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE,
		func(tx repositories.Transaction) (models.User, error) {

			// cleanup spaces
			createUser.Email = strings.TrimSpace(createUser.Email)
			// lowercase email to maintain uniqueness
			createUser.Email = strings.ToLower(createUser.Email)

			createdUserUuid, err := usecase.userRepository.CreateUser(tx, createUser)
			if err != nil {
				return models.User{}, err
			}
			return usecase.userRepository.UserByUid(tx, createdUserUuid)
		},
	)
}

func (usecase *UserUseCase) GetAllUsers() ([]models.User, error) {

	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE,
		func(tx repositories.Transaction) ([]models.User, error) {
			return usecase.userRepository.AllUsers(tx)
		},
	)

}
