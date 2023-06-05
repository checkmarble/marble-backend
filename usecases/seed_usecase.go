package usecases

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type SeedUseCase struct {
	transactionFactory     repositories.TransactionFactory
	organizationRepository repositories.OrganizationRepository
	userRepository         repositories.UserRepository
	organizationSeeder     OrganizationSeeder
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

func (usecase *SeedUseCase) SeedZorgOrganization(zorgOrganizationId string) error {

	err := usecase.transactionFactory.Transaction(models.DATABASE_MARBLE, func(tx repositories.Transaction) error {
		err := usecase.organizationRepository.CreateOrganization(
			tx,
			models.CreateOrganizationInput{
				Name:         "Zorg",
				DatabaseName: "Zorg",
			},
			zorgOrganizationId,
		)

		if repositories.IsIsUniqueViolationError(err) {
			return repositories.ErrIgnoreRoolBackError
		}

		return err
	})

	if err != nil {
		return err
	}

	return usecase.organizationSeeder.Seed(zorgOrganizationId)
}
