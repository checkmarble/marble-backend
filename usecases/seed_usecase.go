package usecases

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"
)

type SeedUseCase struct {
	transactionFactory     repositories.TransactionFactory
	userRepository         repositories.UserRepository
	organizationCreator    organization.OrganizationCreator
	organizationRepository repositories.OrganizationRepository
}

func (usecase *SeedUseCase) SeedMarbleAdmins(firstMarbleAdminEmail string) error {

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
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

	_, err := usecase.organizationCreator.CreateOrganizationWithId(
		zorgOrganizationId,
		models.CreateOrganizationInput{
			Name:         "Zorg",
			DatabaseName: "zorg",
		},
	)
	if repositories.IsIsUniqueViolationError(err) {
		err = nil
	}

	if err != nil {
		return err
	}

	var testBucketName = "marble-backend-export-scheduled-execution-test"
	return usecase.organizationRepository.UpdateOrganization(nil, models.UpdateOrganizationInput{
		ID:                         zorgOrganizationId,
		ExportScheduledExecutionS3: &testBucketName,
	})
}
