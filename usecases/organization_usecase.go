package usecases

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/pkg/errors"
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
	license                      models.LicenseValidation
}

func NewOrganizationUseCase(
	enforceSecurity security.EnforceSecurityOrganization,
	transactionFactory executor_factory.TransactionFactory,
	organizationRepository repositories.OrganizationRepository,
	datamodelRepository repositories.DataModelRepository,
	userRepository repositories.UserRepository,
	organizationCreator organization.OrganizationCreator,
	organizationSchemaRepository repositories.OrganizationSchemaRepository,
	executorFactory executor_factory.ExecutorFactory,
	license models.LicenseValidation,
) OrganizationUseCase {
	return OrganizationUseCase{
		enforceSecurity:              enforceSecurity,
		transactionFactory:           transactionFactory,
		organizationRepository:       organizationRepository,
		datamodelRepository:          datamodelRepository,
		userRepository:               userRepository,
		organizationCreator:          organizationCreator,
		organizationSchemaRepository: organizationSchemaRepository,
		executorFactory:              executorFactory,
		license:                      license,
	}
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
	if organization.DefaultScenarioTimezone != nil {
		_, err := time.LoadLocation(*organization.DefaultScenarioTimezone)
		if err != nil {
			return models.Organization{}, errors.Wrapf(models.BadParameterError,
				"Invalid timezone %s", *organization.DefaultScenarioTimezone)
		}
	}

	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Organization, error) {
		org, err := usecase.organizationRepository.GetOrganizationById(ctx, tx, organization.Id)
		if err != nil {
			return models.Organization{}, err
		}

		if err := usecase.enforceSecurity.EditOrganization(org); err != nil {
			return models.Organization{}, err
		}

		err = usecase.organizationRepository.UpdateOrganization(ctx, tx, organization)
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

func (usecase *OrganizationUseCase) GetOrganizationFeatureAccess(ctx context.Context,
	organizationId string,
) (models.OrganizationFeatureAccess, error) {
	if err := usecase.enforceSecurity.ReadOrganization(organizationId); err != nil {
		return models.OrganizationFeatureAccess{}, err
	}

	dbStoredFeatureAccess, err := usecase.organizationRepository.GetOrganizationFeatureAccess(ctx,
		usecase.executorFactory.NewExecutor(), organizationId)
	if err != nil {
		return models.OrganizationFeatureAccess{}, err
	}

	return dbStoredFeatureAccess.MergeWithLicenseEntitlement(
		&usecase.license.LicenseEntitlements), nil
}

func (usecase *OrganizationUseCase) UpdateOrganizationFeatureAccess(
	ctx context.Context,
	featureAccess models.UpdateOrganizationFeatureAccessInput,
) error {
	if err := usecase.enforceSecurity.CreateOrganization(); err != nil {
		return err
	}
	return usecase.organizationRepository.UpdateOrganizationFeatureAccess(ctx,
		usecase.executorFactory.NewExecutor(), featureAccess)
}
