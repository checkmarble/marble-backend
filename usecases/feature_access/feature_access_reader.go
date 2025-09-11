package feature_access

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type FeatureAccessReaderRepository interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		executor repositories.Executor,
		organizationId string,
	) (models.DbStoredOrganizationFeatureAccess, error)
	UserById(ctx context.Context, exec repositories.Executor, userId string) (models.User, error)
}

type FeatureAccessReader struct {
	enforceSecurity       security.EnforceSecurityOrganization
	repository            FeatureAccessReaderRepository
	executorFactory       executor_factory.ExecutorFactory
	license               models.LicenseValidation
	featuresConfiguration models.FeaturesConfiguration
}

func NewFeatureAccessReader(
	enforceSecurity security.EnforceSecurityOrganization,
	repository FeatureAccessReaderRepository,
	executorFactory executor_factory.ExecutorFactory,
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
	organizationId string,
	userId *models.UserId,
) (models.OrganizationFeatureAccess, error) {
	if err := f.enforceSecurity.ReadOrganization(organizationId); err != nil {
		return models.OrganizationFeatureAccess{}, err
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

	return dbStoredFeatureAccess.MergeWithLicenseEntitlement(f.license.LicenseEntitlements,
		f.featuresConfiguration, user), nil
}
