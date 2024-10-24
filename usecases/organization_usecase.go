package usecases

import (
	"context"

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
	executorFactory              executor_factory.ExecutorFactory
}

func (usecase *OrganizationUseCase) GetOrganizations(ctx context.Context) ([]models.Organization, error) {
	if err := usecase.enforceSecurity.ListOrganization(); err != nil {
		return []models.Organization{}, err
	}
	return usecase.organizationRepository.AllOrganizations(ctx, usecase.executorFactory.NewExecutor())
}

func (usecase *OrganizationUseCase) CreateOrganization(ctx context.Context, name string) (models.Organization, error) {
	if err := usecase.enforceSecurity.CreateOrganization(); err != nil {
		return models.Organization{}, err
	}
	return usecase.organizationCreator.CreateOrganization(ctx, name)
}

func (usecase *OrganizationUseCase) GetOrganization(ctx context.Context, organizationId string) (models.Organization, error) {
	if err := usecase.enforceSecurity.ReadOrganization(organizationId); err != nil {
		return models.Organization{}, err
	}
	return usecase.organizationRepository.GetOrganizationById(ctx,
		usecase.executorFactory.NewExecutor(), organizationId)
}

func (usecase *OrganizationUseCase) UpdateOrganization(ctx context.Context,
	organization models.UpdateOrganizationInput,
) (models.Organization, error) {
	if err := usecase.enforceSecurity.CreateOrganization(); err != nil {
		return models.Organization{}, err
	}
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Organization, error) {
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
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		// delete all users
		err := usecase.userRepository.DeleteUsersOfOrganization(ctx, tx, organizationId)
		if err != nil {
			return err
		}

		err = usecase.organizationRepository.DeleteOrganization(ctx, tx, organizationId)
		if err != nil {
			return err
		}

		db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
		if err != nil {
			return err
		}
		return usecase.organizationSchemaRepository.DeleteSchema(ctx, db)
	})
	if err != nil {
		return err
	}

	usecase.organizationRepository.DeleteOrganizationDecisionRulesAsync(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationId,
	)
	return nil
}
