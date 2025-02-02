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
	enforceSecurity security.EnforceSecurityOrganization
	orgRepo         FeatureAccessReaderOrgRepository
	executorFactory executor_factory.ExecutorFactory
	license         models.LicenseValidation
}

func NewFeatureAccessReader(
	enforceSecurity security.EnforceSecurityOrganization,
	orgRepo FeatureAccessReaderOrgRepository,
	executorFactory executor_factory.ExecutorFactory,
	license models.LicenseValidation,
) FeatureAccessReader {
	return FeatureAccessReader{
		enforceSecurity: enforceSecurity,
		orgRepo:         orgRepo,
		executorFactory: executorFactory,
		license:         license,
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

	return dbStoredFeatureAccess.MergeWithLicenseEntitlement(
		&f.license.LicenseEntitlements), nil
}
