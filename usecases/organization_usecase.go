package usecases

import (
	"context"
	"errors"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"

	"github.com/google/uuid"
)

type OrganizationUseCase struct {
	transactionFactory           repositories.TransactionFactory
	orgTransactionFactory        organization.OrgTransactionFactory
	organizationRepository       repositories.OrganizationRepository
	datamodelRepository          repositories.DataModelRepository
	apiKeyRepository             repositories.ApiKeyRepository
	userRepository               repositories.UserRepository
	organizationCreator          organization.OrganizationCreator
	organizationSchemaRepository repositories.OrganizationSchemaRepository
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
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Organization, error) {
		err := usecase.organizationRepository.UpdateOrganization(tx, organization)
		if err != nil {
			return models.Organization{}, err
		}
		return usecase.organizationRepository.GetOrganizationById(tx, organization.ID)
	})
}

func (usecase *OrganizationUseCase) DeleteOrganization(ctx context.Context, organizationID string) error {

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		// delete all users
		err := usecase.userRepository.DeleteUsersOfOrganization(tx, organizationID)
		if err != nil {
			return err
		}

		// fetch client tables to get schema name, then delete schema
		schema, err := usecase.organizationSchemaRepository.OrganizationSchemaOfOrganization(tx, organizationID)

		schemaFound := err == nil

		if errors.Is(err, models.NotFoundError) {
			// ignore client tables not found: the organization can be older than the introduction of client tables
			err = nil
		}

		if err != nil {
			return err
		}

		if schemaFound {
			// another transaction in client's database to delete client's schema:
			err = usecase.orgTransactionFactory.TransactionInOrgSchema(organizationID, func(clientTx repositories.Transaction) error {
				return usecase.organizationSchemaRepository.DeleteSchema(clientTx, schema.DatabaseSchema.Schema)
			})
			if err != nil {
				return err
			}
		}

		return usecase.organizationRepository.DeleteOrganization(nil, organizationID)
	})
}

func (usecase *OrganizationUseCase) GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error) {
	return usecase.datamodelRepository.GetDataModel(ctx, organizationID)
}

func (usecase *OrganizationUseCase) GetUsersOfOrganization(organizationIDFilter string) ([]models.User, error) {

	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.User, error) {
			return usecase.userRepository.UsersOfOrganization(tx, organizationIDFilter)
		},
	)
}

func (usecase *OrganizationUseCase) GetApiKeysOfOrganization(ctx context.Context, organizationId string) ([]models.ApiKey, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) ([]models.ApiKey, error) {
		apiKey, err := usecase.apiKeyRepository.GetApiKeysOfOrganization(tx, organizationId)
		if err != nil {
			return []models.ApiKey{}, err
		}
		return apiKey, nil
	})
}
