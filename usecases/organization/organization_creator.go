package organization

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type OrganizationCreator struct {
	CustomListRepository   repositories.CustomListRepository
	ExecutorFactory        executor_factory.ExecutorFactory
	TransactionFactory     executor_factory.TransactionFactory
	OrganizationRepository repositories.OrganizationRepository
}

func (creator *OrganizationCreator) CreateOrganization(ctx context.Context,
	input models.CreateOrganizationInput,
) (models.Organization, error) {
	newOrganizationId := uuid.New()
	organization, err := executor_factory.TransactionReturnValue(ctx,
		creator.TransactionFactory, func(tx repositories.Transaction) (models.Organization, error) {
			if err := creator.OrganizationRepository.CreateOrganization(
				ctx,
				tx,
				newOrganizationId,
				input); err != nil {
				return models.Organization{}, err
			}
			return creator.OrganizationRepository.GetOrganizationById(ctx, tx, newOrganizationId)
		})
	if err != nil {
		return models.Organization{}, err
	}

	if err = creator.seedDefaultList(ctx, organization.Id); err != nil {
		return models.Organization{}, err
	}

	return organization, nil
}

func (creator *OrganizationCreator) seedDefaultList(ctx context.Context, organizationId uuid.UUID) error {
	logger := utils.LoggerFromContext(ctx)
	exec := creator.ExecutorFactory.NewExecutor()
	newCustomListId := uuid.NewString()

	err := creator.CustomListRepository.CreateCustomList(ctx, exec, models.CreateCustomListInput{
		Name:           "Welcome to Marble",
		Description:    "Need a whitelist or blacklist ? The list is your friend :)",
		Kind:           models.CustomListText,
		OrganizationId: organizationId,
	}, newCustomListId)
	if err != nil {
		return err
	}

	addCustomListValueInput := models.AddCustomListValueInput{
		CustomListId: newCustomListId,
		Value:        "Welcome",
	}
	_ = creator.CustomListRepository.AddCustomListValue(ctx, exec, models.CustomListText, addCustomListValueInput, uuid.NewString(), nil)
	addCustomListValueInput.Value = "to"
	_ = creator.CustomListRepository.AddCustomListValue(ctx, exec, models.CustomListText, addCustomListValueInput, uuid.NewString(), nil)
	addCustomListValueInput.Value = "marble"
	_ = creator.CustomListRepository.AddCustomListValue(ctx, exec, models.CustomListText, addCustomListValueInput, uuid.NewString(), nil)

	logger.InfoContext(ctx, "Finish to create the default custom list for the organization")
	return nil
}
