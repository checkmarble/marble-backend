package usecases

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type OrganizationUseCase struct {
	enforceSecurity              security.EnforceSecurityOrganization
	transactionFactory           executor_factory.TransactionFactory
	organizationRepository       repositories.OrganizationRepository
	datamodelRepository          repositories.DataModelRepository
	userRepository               repositories.UserRepository
	organizationCreator          organization.OrganizationCreator
	organizationSchemaRepository repositories.OrganizationSchemaRepository
	populateOrganizationSchema   organization.PopulateOrganizationSchema
	clientSchemaExecutorFactory  executor_factory.ClientSchemaExecutorFactory
}

func (usecase *OrganizationUseCase) GetOrganizations(ctx context.Context) ([]models.Organization, error) {
	if err := usecase.enforceSecurity.ListOrganization(); err != nil {
		return []models.Organization{}, err
	}
	return usecase.organizationRepository.AllOrganizations(ctx, nil)
}

func (usecase *OrganizationUseCase) CreateOrganization(ctx context.Context, createOrga models.CreateOrganizationInput) (models.Organization, error) {
	if err := usecase.enforceSecurity.CreateOrganization(); err != nil {
		return models.Organization{}, err
	}
	newOrganizationId := uuid.NewString()
	return usecase.organizationCreator.CreateOrganizationWithId(ctx, newOrganizationId, createOrga)
}

func (usecase *OrganizationUseCase) GetOrganization(ctx context.Context, organizationId string) (models.Organization, error) {
	if err := usecase.enforceSecurity.ReadOrganization(organizationId); err != nil {
		return models.Organization{}, err
	}
	return usecase.organizationRepository.GetOrganizationById(ctx, nil, organizationId)
}

func (usecase *OrganizationUseCase) UpdateOrganization(ctx context.Context, organization models.UpdateOrganizationInput) (models.Organization, error) {
	if err := usecase.enforceSecurity.CreateOrganization(); err != nil {
		return models.Organization{}, err
	}
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Executor) (models.Organization, error) {
		err := usecase.organizationRepository.UpdateOrganization(ctx, tx, organization)
		if err != nil {
			return models.Organization{}, err
		}
		return usecase.organizationRepository.GetOrganizationById(ctx, tx, organization.Id)
	})
}

func (usecase *OrganizationUseCase) DeleteOrganization(ctx context.Context, organizationId string) error {
	if err := usecase.enforceSecurity.DeleteOrganization(); err != nil {
		return err
	}
	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		// delete all users
		err := usecase.userRepository.DeleteUsersOfOrganization(ctx, tx, organizationId)
		if err != nil {
			return err
		}

		// fetch client tables to get schema name, then delete schema
		schema, err := usecase.organizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, tx, organizationId)

		schemaFound := err == nil

		if errors.Is(err, models.NotFoundError) {
			// ignore client tables not found: the organization can be older than the introduction of client tables
			err = nil
		}

		if err != nil {
			return err
		}

		if schemaFound {
			db, err := usecase.clientSchemaExecutorFactory.NewClientDbExecutor(ctx, organizationId)
			if err != nil {
				return err
			}
			// another transaction in client's database to delete client's schema:
			return usecase.organizationSchemaRepository.DeleteSchema(ctx, db, schema.DatabaseSchema.Schema)
		}

		return usecase.organizationRepository.DeleteOrganization(ctx, tx, organizationId)
	})
}

func (usecase *OrganizationUseCase) GetUsersOfOrganization(ctx context.Context, organizationIDFilter string) ([]models.User, error) {
	if err := usecase.enforceSecurity.ReadOrganization(organizationIDFilter); err != nil {
		return []models.User{}, err
	}
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) ([]models.User, error) {
			return usecase.userRepository.UsersOfOrganization(ctx, tx, organizationIDFilter)
		},
	)
}
