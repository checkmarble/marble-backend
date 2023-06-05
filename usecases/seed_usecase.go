package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type SeedUseCase struct {
	transactionFactory     repositories.TransactionFactory
	organizationRepository repositories.OrganizationRepository
	userRepository         repositories.UserRepository
}

func (usecase *SeedUseCase) SeedMarbleAdmins(firstMarbleAdminEmail string) error {

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE, func(tx repositories.Transaction) error {
		_, err := usecase.userRepository.CreateUser(tx, models.CreateUser{
			Email: firstMarbleAdminEmail,
			Role:  models.MARBLE_ADMIN,
		})

		// ignore user already added
		if repositories.IsIsUniqueViolationError(err) {
			return repositories.ErrIgnoreRoolBackError
		}
		return err
	})
}

func (usecase *SeedUseCase) SeedZorgOrganization() error {

	ctx := context.Background()

	_, err := usecase.organizationRepository.CreateOrganization(ctx, models.CreateOrganizationInput{
		Name:         "Zorg",
		DatabaseName: "Zorg",
	})

	if err != nil {
		return err
	}
	return nil
}
