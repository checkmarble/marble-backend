package usecases

import (
	"context"
	"net"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

type OrganizationUsecaseFeatureAccessReader interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		organizationId uuid.UUID,
		user *models.UserId,
	) (models.OrganizationFeatureAccess, error)
}

type organizationUsecaseScreeningChecksRepository interface {
	HasScreeningConfigs(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) (bool, error)
	GetContinuousScreeningConfigsByOrgId(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) ([]models.ContinuousScreeningConfig, error)
}

type OrganizationUseCase struct {
	enforceSecurity              security.EnforceSecurityOrganization
	transactionFactory           executor_factory.TransactionFactory
	organizationRepository       repositories.OrganizationRepository
	datamodelRepository          repositories.DataModelRepository
	userRepository               repositories.UserRepository
	organizationCreator          organization.OrganizationCreator
	organizationSchemaRepository repositories.OrganizationSchemaRepository
	executorFactory              executor_factory.ExecutorFactory
	featureAccessReader          OrganizationUsecaseFeatureAccessReader
	screeningConfigRepository    organizationUsecaseScreeningChecksRepository
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
	featureAccessReader OrganizationUsecaseFeatureAccessReader,
	screeningConfigRepository organizationUsecaseScreeningChecksRepository,
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
		featureAccessReader:          featureAccessReader,
		screeningConfigRepository:    screeningConfigRepository,
	}
}

func (usecase *OrganizationUseCase) GetOrganizations(ctx context.Context) ([]models.Organization, error) {
	if err := usecase.enforceSecurity.ListOrganization(); err != nil {
		return []models.Organization{}, err
	}
	return usecase.organizationRepository.AllOrganizations(ctx, usecase.executorFactory.NewExecutor())
}

func (usecase *OrganizationUseCase) CreateOrganization(
	ctx context.Context,
	input models.CreateOrganizationInput,
) (models.Organization, error) {
	if err := usecase.enforceSecurity.CreateOrganization(); err != nil {
		return models.Organization{}, err
	}
	return usecase.organizationCreator.CreateOrganization(ctx, input)
}

func (usecase *OrganizationUseCase) GetOrganization(ctx context.Context, organizationId uuid.UUID) (models.Organization, error) {
	if err := usecase.enforceSecurity.ReadOrganization(organizationId); err != nil {
		return models.Organization{}, err
	}
	return usecase.organizationRepository.GetOrganizationById(ctx,
		usecase.executorFactory.NewExecutor(), organizationId)
}

func (usecase *OrganizationUseCase) UpdateOrganization(
	ctx context.Context,
	orgId uuid.UUID,
	organization models.UpdateOrganizationInput,
) (models.Organization, error) {
	if organization.DefaultScenarioTimezone != nil {
		_, err := time.LoadLocation(*organization.DefaultScenarioTimezone)
		if err != nil {
			return models.Organization{}, errors.Wrapf(models.BadParameterError,
				"Invalid timezone %s", *organization.DefaultScenarioTimezone)
		}
	}

	if organization.ScreeningConfig.MatchThreshold != nil {
		if *organization.ScreeningConfig.MatchThreshold < 0 ||
			*organization.ScreeningConfig.MatchThreshold > 100 {
			return models.Organization{}, errors.Wrapf(models.BadParameterError,
				"threshold should be between 0 and 100")
		}
	}

	if organization.ScreeningConfig.Providers != nil {
		featureAccess, err := usecase.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
		if err != nil {
			return models.Organization{}, err
		}

		for _, provider := range organization.ScreeningConfig.Providers {
			if provider == "lexisnexis" {
				if !featureAccess.LexisNexis.IsAllowed() {
					return models.Organization{}, errors.Wrap(models.ForbiddenError,
						"organization does not have access to the Lexis Nexis screening provider")
				}
			}
		}
	}

	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.Organization, error) {
		org, err := usecase.organizationRepository.GetOrganizationById(ctx, tx, orgId)
		if err != nil {
			return models.Organization{}, err
		}

		if err := usecase.enforceSecurity.EditOrganization(org); err != nil {
			return models.Organization{}, err
		}

		for feature, provider := range organization.ScreeningConfig.Providers {
			if org.GetScreeningProviderFor(models.ScreeningFeature(feature)) != provider {
				switch feature {
				case string(models.ScreeningFeatureTransactionMonitoring):
					configExists, err := usecase.screeningConfigRepository.HasScreeningConfigs(ctx, tx, org.Id)
					if err != nil {
						return models.Organization{}, err
					}
					if configExists {
						return models.Organization{}, errors.Wrap(models.UnprocessableEntityError,
							"cannot change transaction monitoring screening provider because a scenario already uses screening")
					}

				case string(models.ScreeningFeatureContinuousMonitoring):
					configs, err := usecase.screeningConfigRepository.GetContinuousScreeningConfigsByOrgId(ctx, tx, org.Id)
					if err != nil {
						return models.Organization{}, err
					}
					if len(configs) > 0 {
						return models.Organization{}, errors.Wrap(models.UnprocessableEntityError,
							"cannot change continuous monitoring screening provider because a scenario already uses screening")
					}

				default:
					// Other features can change their provider whenever required.
				}
			}
		}

		err = usecase.organizationRepository.UpdateOrganization(ctx, tx, orgId, organization)
		if err != nil {
			return models.Organization{}, err
		}
		return usecase.organizationRepository.GetOrganizationById(ctx, tx, orgId)
	})
}

func (usecase *OrganizationUseCase) DeleteOrganization(ctx context.Context, organizationId uuid.UUID) error {
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

func (usecase *OrganizationUseCase) GetOrganizationFeatureAccess(
	ctx context.Context,
	organizationId uuid.UUID,
	userId *models.UserId,
) (models.OrganizationFeatureAccess, error) {
	return usecase.featureAccessReader.GetOrganizationFeatureAccess(ctx, organizationId, userId)
}

func (usecase *OrganizationUseCase) UpdateOrganizationFeatureAccess(
	ctx context.Context,
	featureAccess models.UpdateOrganizationFeatureAccessInput,
) error {
	if err := usecase.enforceSecurity.CreateOrganization(); err != nil {
		return err
	}

	cache := usecase.executorFactory.NewExecutor().Cache(ctx).WithOrg(featureAccess.OrganizationId)

	if err := cache.DeletePrefix(ctx, cache.Key("feature-access", "*")); err != nil {
		return err
	}
	if err := cache.Exec(func(c *redis.Client) error {
		return c.Del(ctx, cache.Key("feature-access")).Err()
	}); err != nil {
		return err
	}

	return usecase.organizationRepository.UpdateOrganizationFeatureAccess(ctx,
		usecase.executorFactory.NewExecutor(), featureAccess)
}

var (
	ErrClientOutsideOfAllowedNetworks = errors.New("client is outside of new IP whitelist")
	ErrRealClientIpNotPresent         = errors.New("no value for client IP in x-real-ip header")
)

func (usecase OrganizationUseCase) UpdateOrganizationSubnets(ctx context.Context, subnets []net.IPNet) ([]net.IPNet, error) {
	orgId := usecase.enforceSecurity.OrgId()

	org, err := usecase.organizationRepository.GetOrganizationById(ctx,
		usecase.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return nil, err
	}
	if err := usecase.enforceSecurity.EditOrganization(org); err != nil {
		return nil, err
	}

	if len(subnets) > 0 {
		clientIp, ok := ctx.Value(utils.ContextKeyClientIp).(net.IP)

		if !ok {
			return nil, ErrRealClientIpNotPresent
		}

		found := false

		for _, subnet := range subnets {
			if subnet.Contains(clientIp) {
				found = true
				break
			}
		}
		if !found {
			return nil, ErrClientOutsideOfAllowedNetworks
		}
	}

	subnets, err = usecase.organizationRepository.UpdateOrganizationAllowedNetworks(ctx,
		usecase.executorFactory.NewExecutor(), orgId, subnets)
	if err != nil {
		return nil, errors.Wrap(err, "could not update whitelisted subnets")
	}

	return subnets, nil
}
