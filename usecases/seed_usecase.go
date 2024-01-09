package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/transaction"

	"github.com/google/uuid"
)

type SeedUseCase struct {
	transactionFactory     transaction.TransactionFactory
	userRepository         repositories.UserRepository
	organizationCreator    organization.OrganizationCreator
	organizationRepository repositories.OrganizationRepository
	customListRepository   repositories.CustomListRepository
}

func (usecase *SeedUseCase) SeedMarbleAdmins(ctx context.Context, firstMarbleAdminEmail string) error {

	return usecase.transactionFactory.Transaction(ctx, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		_, err := usecase.userRepository.CreateUser(ctx, tx, models.CreateUser{
			Email: firstMarbleAdminEmail,
			Role:  models.MARBLE_ADMIN,
		})

		// ignore user already added
		if repositories.IsUniqueViolationError(err) {
			return repositories.ErrIgnoreRoolBackError
		}
		return err
	})

}

func (usecase *SeedUseCase) SeedZorgOrganization(ctx context.Context, zorgOrganizationId string) error {

	_, err := usecase.organizationCreator.CreateOrganizationWithId(
		ctx,
		zorgOrganizationId,
		models.CreateOrganizationInput{
			Name:         "Zorg",
			DatabaseName: "zorg",
		},
	)
	if repositories.IsUniqueViolationError(err) {
		err = nil
	}

	if err != nil {
		return err
	}

	// assign test s3 bucket name to zorg organization
	var testBucketName = "marble-backend-export-scheduled-execution-test"
	err = usecase.organizationRepository.UpdateOrganization(ctx, nil, models.UpdateOrganizationInput{
		Id:                         zorgOrganizationId,
		ExportScheduledExecutionS3: &testBucketName,
	})

	if err != nil {
		return err
	}

	// add Admin user Jean-Baptiste Emanuel Zorg
	_, err = usecase.userRepository.CreateUser(ctx, nil, models.CreateUser{
		Email:          "jbe@zorg.com", // Jean-Baptiste Emanuel Zorg
		Role:           models.ADMIN,
		OrganizationId: zorgOrganizationId,
		FirstName:      "Jean-Baptiste Emanuel",
		LastName:       "Zorg",
	})
	if repositories.IsUniqueViolationError(err) {
		err = nil
	}
	if err != nil {
		return err
	}

	newCustomListId := "d6643d7e-c973-4899-a9a8-805f868ef90a"

	err = usecase.customListRepository.CreateCustomList(ctx, nil, models.CreateCustomListInput{
		Name:        "zorg custom list",
		Description: "Need a whitelist or blacklist ? The list is your friend :)",
	}, zorgOrganizationId, newCustomListId)

	if err == nil {
		// add some values to the hardcoded custom list
		addCustomListValueInput := models.AddCustomListValueInput{
			CustomListId: newCustomListId,
			Value:        "Welcome",
		}
		usecase.customListRepository.AddCustomListValue(ctx, nil, addCustomListValueInput, uuid.NewString())
		addCustomListValueInput.Value = "to"
		usecase.customListRepository.AddCustomListValue(ctx, nil, addCustomListValueInput, uuid.NewString())
		addCustomListValueInput.Value = "marble"
		usecase.customListRepository.AddCustomListValue(ctx, nil, addCustomListValueInput, uuid.NewString())
	}

	if repositories.IsUniqueViolationError(err) {
		err = nil
	}

	if err != nil {
		return err
	}

	// reset firebase id of all users, so when the firebase emulator restarts
	return usecase.transactionFactory.Transaction(ctx, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		users, err := usecase.userRepository.AllUsers(ctx, tx)
		if err != nil {
			return err
		}

		for _, user := range users {
			err = usecase.userRepository.UpdateFirebaseId(ctx, tx, user.UserId, "")
			if err != nil {
				return err
			}
		}
		return nil
	})
}
