package usecases

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type OrganizationUseCase struct {
	enforceSecurity              security.EnforceSecurityOrganization
	transactionFactory           transaction.TransactionFactory
	orgTransactionFactory        transaction.Factory
	organizationRepository       repositories.OrganizationRepository
	datamodelRepository          repositories.DataModelRepository
	apiKeyRepository             repositories.ApiKeyRepository
	userRepository               repositories.UserRepository
	organizationCreator          organization.OrganizationCreator
	organizationSchemaRepository repositories.OrganizationSchemaRepository
	populateOrganizationSchema   organization.PopulateOrganizationSchema
}

func (usecase *OrganizationUseCase) GetOrganizations(ctx context.Context) ([]models.Organization, error) {
	if err := usecase.enforceSecurity.ListOrganization(); err != nil {
		return []models.Organization{}, err
	}
	return usecase.organizationRepository.AllOrganizations(nil)
}

func (usecase *OrganizationUseCase) CreateOrganization(ctx context.Context, createOrga models.CreateOrganizationInput) (models.Organization, error) {
	if err := usecase.enforceSecurity.CreateOrganization(); err != nil {
		return models.Organization{}, err
	}
	newOrganizationId := uuid.NewString()
	return usecase.organizationCreator.CreateOrganizationWithId(newOrganizationId, createOrga)
}

func (usecase *OrganizationUseCase) GetOrganization(ctx context.Context, organizationId string) (models.Organization, error) {
	if err := usecase.enforceSecurity.ReadOrganization(organizationId); err != nil {
		return models.Organization{}, err
	}
	return usecase.organizationRepository.GetOrganizationById(nil, organizationId)
}

func (usecase *OrganizationUseCase) UpdateOrganization(ctx context.Context, organization models.UpdateOrganizationInput) (models.Organization, error) {
	if err := usecase.enforceSecurity.CreateOrganization(); err != nil {
		return models.Organization{}, err
	}
	return transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Organization, error) {
		err := usecase.organizationRepository.UpdateOrganization(tx, organization)
		if err != nil {
			return models.Organization{}, err
		}
		return usecase.organizationRepository.GetOrganizationById(tx, organization.Id)
	})
}

func (usecase *OrganizationUseCase) DeleteOrganization(ctx context.Context, organizationId string) error {
	if err := usecase.enforceSecurity.DeleteOrganization(); err != nil {
		return err
	}
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		// delete all users
		err := usecase.userRepository.DeleteUsersOfOrganization(tx, organizationId)
		if err != nil {
			return err
		}

		// fetch client tables to get schema name, then delete schema
		schema, err := usecase.organizationSchemaRepository.OrganizationSchemaOfOrganization(tx, organizationId)

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
			err = usecase.orgTransactionFactory.TransactionInOrgSchema(organizationId, func(clientTx repositories.Transaction) error {
				return usecase.organizationSchemaRepository.DeleteSchema(clientTx, schema.DatabaseSchema.Schema)
			})
			if err != nil {
				return err
			}
		}

		return usecase.organizationRepository.DeleteOrganization(nil, organizationId)
	})
}

func (usecase *OrganizationUseCase) GetUsersOfOrganization(organizationIDFilter string) ([]models.User, error) {
	if err := usecase.enforceSecurity.ListOrganization(); err != nil {
		return []models.User{}, err
	}
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.User, error) {
			return usecase.userRepository.UsersOfOrganization(tx, organizationIDFilter)
		},
	)
}

func (usecase *OrganizationUseCase) GetApiKeysOfOrganization(ctx context.Context, organizationId string) ([]models.ApiKey, error) {
	return transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) ([]models.ApiKey, error) {
		apiKeys, err := usecase.apiKeyRepository.GetApiKeysOfOrganization(tx, organizationId)
		if err != nil {
			return []models.ApiKey{}, err
		}
		for _, ak := range apiKeys {
			if err := usecase.enforceSecurity.ReadOrganizationApiKeys(ak); err != nil {
				return []models.ApiKey{}, err
			}
		}
		return apiKeys, nil
	})
}
