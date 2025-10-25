package feature_access

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type FeatureAccessReaderRepository interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		executor repositories.Executor,
		organizationId uuid.UUID,
	) (models.DbStoredOrganizationFeatureAccess, error)
	UserById(ctx context.Context, exec repositories.Executor, userId string) (models.User, error)
}

type FeatureAccessReader struct {
	enforceSecurity       security.EnforceSecurityOrganization
	repository            FeatureAccessReaderRepository
	cache                 *repositories.RedisClient
	executorFactory       executor_factory.ExecutorFactory
	license               models.LicenseValidation
	featuresConfiguration models.FeaturesConfiguration
}

func NewFeatureAccessReader(
	enforceSecurity security.EnforceSecurityOrganization,
	repository FeatureAccessReaderRepository,
	executorFactory executor_factory.ExecutorFactory,
	redis *repositories.RedisClient,
	license models.LicenseValidation,
	hasConvoyServerSetup bool,
	hasMetabaseSetup bool,
	hasOpensanctionsSetup bool,
	hasNameRecognitionSetup bool,
) FeatureAccessReader {
	return FeatureAccessReader{
		enforceSecurity: enforceSecurity,
		repository:      repository,
		executorFactory: executorFactory,
		cache:           redis,
		license:         license,
		featuresConfiguration: models.FeaturesConfiguration{
			Webhooks:        hasConvoyServerSetup,
			Sanctions:       hasOpensanctionsSetup,
			NameRecognition: hasNameRecognitionSetup,
			Analytics:       hasMetabaseSetup,
		},
	}
}

func (f FeatureAccessReader) GetOrganizationFeatureAccess(
	ctx context.Context,
	organizationId uuid.UUID,
	userId *models.UserId,
) (models.OrganizationFeatureAccess, error) {
	cache := f.cache.NewExecutor(organizationId)

	cacheKey := cache.Key("feature-access")
	if userId != nil {
		cacheKey = cache.Key("feature-access", "user", string(*userId))
	}

	if err := f.enforceSecurity.ReadOrganization(organizationId); err != nil {
		return models.OrganizationFeatureAccess{}, err
	}

	if entitlements, err := repositories.RedisLoadMap[models.OrganizationFeatureAccess](ctx, cache, cacheKey); err == nil {
		return entitlements, nil
	}

	dbStoredFeatureAccess, err := f.repository.GetOrganizationFeatureAccess(ctx,
		f.executorFactory.NewExecutor(), organizationId)
	if err != nil {
		return models.OrganizationFeatureAccess{}, err
	}

	var user *models.User
	if userId != nil {
		u, err := f.repository.UserById(ctx, f.executorFactory.NewExecutor(), string(*userId))
		if err != nil {
			return models.OrganizationFeatureAccess{}, err
		}
		user = &u
	}

	entitlements := dbStoredFeatureAccess.MergeWithLicenseEntitlement(f.license.LicenseEntitlements,
		f.featuresConfiguration, user)

	cache.Tx(ctx, func(c redis.Pipeliner) error {
		if err := c.HSet(ctx, cacheKey, entitlements).Err(); err != nil {
			return err
		}

		return c.Expire(ctx, cacheKey, time.Hour).Err()
	})

	return entitlements, nil
}
