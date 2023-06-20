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

	// assign test s3 bucket name to zorg organization
	var testBucketName = "marble-backend-export-scheduled-execution-test"
	err = usecase.organizationRepository.UpdateOrganization(nil, models.UpdateOrganizationInput{
		ID:                         zorgOrganizationId,
		ExportScheduledExecutionS3: &testBucketName,
	})

	if err != nil {
		return err
	}

	// add Admin user Jean-Baptiste Emanuel Zorg
	jbeUserId, err := usecase.userRepository.CreateUser(nil, models.CreateUser{
		Email:          "jbe@zorg.com", // Jean-Baptiste Emanuel Zorg
		Role:           models.ADMIN,
		OrganizationId: zorgOrganizationId,
	})
	if repositories.IsIsUniqueViolationError(err) {
		err = nil
	}
	if err != nil {
		return err
	}

	err = usecase.userRepository.UpdateFirebaseId(nil, jbeUserId, "")

	return err
}
