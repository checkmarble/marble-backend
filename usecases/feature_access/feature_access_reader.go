package feature_access

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type FeatureAccessReaderOrgRepository interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		executor repositories.Executor,
		organizationId string,
	) (models.DbStoredOrganizationFeatureAccess, error)
}

type FeatureAccessReader struct {
	enforceSecurity       security.EnforceSecurityOrganization
	orgRepo               FeatureAccessReaderOrgRepository
	executorFactory       executor_factory.ExecutorFactory
	license               models.LicenseValidation
	featuresConfiguration models.FeaturesConfiguration
	hasTestMode           bool
}

func NewFeatureAccessReader(
	enforceSecurity security.EnforceSecurityOrganization,
	orgRepo FeatureAccessReaderOrgRepository,
	executorFactory executor_factory.ExecutorFactory,
	license models.LicenseValidation,
	hasConvoyServerSetup bool,
	hasMetabaseSetup bool,
	hasOpensanctionsSetup bool,
	hasTestMode bool,
) FeatureAccessReader {
	return FeatureAccessReader{
		enforceSecurity: enforceSecurity,
		orgRepo:         orgRepo,
		executorFactory: executorFactory,
		license:         license,
		featuresConfiguration: models.FeaturesConfiguration{
			Webhooks:  hasConvoyServerSetup,
			Sanctions: hasOpensanctionsSetup,
			Analytics: hasMetabaseSetup,
		},
		hasTestMode: hasTestMode,
	}
}

func (f FeatureAccessReader) GetOrganizationFeatureAccess(
	ctx context.Context,
	organizationId string,
) (models.OrganizationFeatureAccess, error) {
	if err := f.enforceSecurity.ReadOrganization(organizationId); err != nil {
		return models.OrganizationFeatureAccess{}, err
	}

	dbStoredFeatureAccess, err := f.orgRepo.GetOrganizationFeatureAccess(ctx,
		f.executorFactory.NewExecutor(), organizationId)
	if err != nil {
		return models.OrganizationFeatureAccess{}, err
	}

	return dbStoredFeatureAccess.MergeWithLicenseEntitlement(f.license.LicenseEntitlements,
		f.featuresConfiguration, f.hasTestMode), nil
}
