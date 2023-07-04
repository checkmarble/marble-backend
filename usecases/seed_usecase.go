package usecases

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"

	"github.com/google/uuid"
)

type SeedUseCase struct {
	transactionFactory     repositories.TransactionFactory
	userRepository         repositories.UserRepository
	organizationCreator    organization.OrganizationCreator
	organizationRepository repositories.OrganizationRepository
	customListRepository   repositories.CustomListRepository
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
	_, err = usecase.userRepository.CreateUser(nil, models.CreateUser{
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

	newCustomListId := uuid.NewString()

	err = usecase.customListRepository.CreateCustomList(nil, models.CreateCustomListInput{
		OrgId:       zorgOrganizationId,
		Name:        "Welcome to Marble",
		Description: "Need a whitelist or blacklist ? The list is your friend :)",
	}, newCustomListId)
	if err != nil {
		return err
	}
	customListValueId1 := uuid.NewString()
	customListValueId2 := uuid.NewString()
	customListValueId3 := uuid.NewString()

	addCustomListValueInput := models.AddCustomListValueInput{
		OrgId:        zorgOrganizationId,
		CustomListId: newCustomListId,
		Value:        "Welcome",
	}
	usecase.customListRepository.AddCustomListValue(nil, addCustomListValueInput, customListValueId1)
	addCustomListValueInput.Value = "to"
	usecase.customListRepository.AddCustomListValue(nil, addCustomListValueInput, customListValueId2)
	addCustomListValueInput.Value = "marble"
	usecase.customListRepository.AddCustomListValue(nil, addCustomListValueInput, customListValueId3)

	// reset firebase id of all users, so when the firebase emulator restarts
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, (func(tx repositories.Transaction) error {

		users, err := usecase.userRepository.AllUsers(tx)
		if err != nil {
			return err
		}

		for _, user := range users {
			err = usecase.userRepository.UpdateFirebaseId(tx, user.UserId, "")
			if err != nil {
				return err
			}

		}
		return nil
	}))
}
