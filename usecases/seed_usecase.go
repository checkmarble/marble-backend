package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/organization"

	"github.com/google/uuid"
)

type SeedUseCase struct {
	transactionFactory     executor_factory.TransactionFactory
	executorFactory        executor_factory.ExecutorFactory
	userRepository         repositories.UserRepository
	organizationCreator    organization.OrganizationCreator
	organizationRepository repositories.OrganizationRepository
	customListRepository   repositories.CustomListRepository
}

func (usecase *SeedUseCase) SeedMarbleAdmins(ctx context.Context, firstMarbleAdminEmail string) error {

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		_, err := usecase.userRepository.CreateUser(ctx, tx, models.CreateUser{
			Email: firstMarbleAdminEmail,
			Role:  models.MARBLE_ADMIN,
		})

		// ignore user already added
		if repositories.IsUniqueViolationError(err) {
			return models.ErrIgnoreRollBackError
		}
		return err
	})

}

func (usecase *SeedUseCase) SeedZorgOrganization(ctx context.Context, zorgOrganizationId string) error {
	exec := usecase.executorFactory.NewExecutor()
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
	err = usecase.organizationRepository.UpdateOrganization(ctx, exec, models.UpdateOrganizationInput{
		Id:                         zorgOrganizationId,
		ExportScheduledExecutionS3: &testBucketName,
	})

	if err != nil {
		return err
	}

	// add Admin user Jean-Baptiste Emanuel Zorg
	_, err = usecase.userRepository.CreateUser(ctx, exec, models.CreateUser{
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

	err = usecase.customListRepository.CreateCustomList(ctx, exec, models.CreateCustomListInput{
		Name:        "zorg custom list",
		Description: "Need a whitelist or blacklist ? The list is your friend :)",
	}, zorgOrganizationId, newCustomListId)

	if err == nil {
		// add some values to the hardcoded custom list
		addCustomListValueInput := models.AddCustomListValueInput{
			CustomListId: newCustomListId,
			Value:        "Welcome",
		}
		usecase.customListRepository.AddCustomListValue(ctx, exec, addCustomListValueInput, uuid.NewString())
		addCustomListValueInput.Value = "to"
		usecase.customListRepository.AddCustomListValue(ctx, exec, addCustomListValueInput, uuid.NewString())
		addCustomListValueInput.Value = "marble"
		usecase.customListRepository.AddCustomListValue(ctx, exec, addCustomListValueInput, uuid.NewString())
	}

	if repositories.IsUniqueViolationError(err) {
		err = nil
	}

	if err != nil {
		return err
	}

	return nil
}
