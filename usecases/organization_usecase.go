package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"

	"github.com/google/uuid"
)

type OrganizationUseCase struct {
	transactionFactory     repositories.TransactionFactory
	organizationRepository repositories.OrganizationRepository
	datamodelRepository    repositories.DataModelRepository
	apiKeyRepository       repositories.ApiKeyRepository
	userRepository         repositories.UserRepository
	organizationCreator    organization.OrganizationCreator
}

func (usecase *OrganizationUseCase) GetOrganizations(ctx context.Context) ([]models.Organization, error) {
	return usecase.organizationRepository.AllOrganizations(nil)
}

func (usecase *OrganizationUseCase) CreateOrganization(ctx context.Context, createOrga models.CreateOrganizationInput) (models.Organization, error) {

	newOrganizationId := uuid.NewString()
	return usecase.organizationCreator.CreateOrganizationWithId(newOrganizationId, createOrga)
}

func (usecase *OrganizationUseCase) GetOrganization(ctx context.Context, organizationID string) (models.Organization, error) {
	return usecase.organizationRepository.GetOrganizationById(nil, organizationID)
}

func (usecase *OrganizationUseCase) UpdateOrganization(ctx context.Context, organization models.UpdateOrganizationInput) (models.Organization, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE, func(tx repositories.Transaction) (models.Organization, error) {
		err := usecase.organizationRepository.UpdateOrganization(tx, organization)
		if err != nil {
			return models.Organization{}, err
		}
		return usecase.organizationRepository.GetOrganizationById(tx, organization.ID)
	})
}

func (usecase *OrganizationUseCase) DeleteOrganization(ctx context.Context, organizationID string) error {

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE, func(tx repositories.Transaction) error {
		return usecase.organizationRepository.DeleteOrganization(nil, organizationID)
	})
}

func (usecase *OrganizationUseCase) GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error) {
	return usecase.datamodelRepository.GetDataModel(ctx, organizationID)
}

func (usecase *OrganizationUseCase) GetUsersOfOrganization(organizationIDFilter string) ([]models.User, error) {

	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE,
		func(tx repositories.Transaction) ([]models.User, error) {
			return usecase.userRepository.UsersOfOrganization(tx, organizationIDFilter)
		},
	)
}

func (usecase *OrganizationUseCase) GetApiKeyOfOrganization(ctx context.Context, organizationId string) ([]models.ApiKey, error) {

	return usecase.apiKeyRepository.GetApiKeyOfOrganization(ctx, organizationId)
}
