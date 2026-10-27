package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type tenantOIDCDiscoveryRepository interface {
	DiscoverTenantOIDCConfigurationByDomain(ctx context.Context, exec repositories.Executor, domain string) (models.TenantOIDCConfiguration, error)
}

type TenantOIDCDiscoveryUsecase struct {
	executorFactory executor_factory.ExecutorFactory
	repository      tenantOIDCDiscoveryRepository
	ssoEnabled      bool
}

func NewTenantOIDCDiscoveryUsecase(executorFactory executor_factory.ExecutorFactory, repository tenantOIDCDiscoveryRepository, ssoEnabled bool) TenantOIDCDiscoveryUsecase {
	return TenantOIDCDiscoveryUsecase{executorFactory: executorFactory, repository: repository, ssoEnabled: ssoEnabled}
}

func (usecase TenantOIDCDiscoveryUsecase) ByDomain(ctx context.Context, domain string) (models.TenantOIDCConfiguration, error) {
	if !usecase.ssoEnabled {
		return models.TenantOIDCConfiguration{}, models.NotFoundError
	}
	domain, err := normalizeTenantDomain(domain)
	if err != nil {
		return models.TenantOIDCConfiguration{}, err
	}
	return usecase.repository.DiscoverTenantOIDCConfigurationByDomain(ctx, usecase.executorFactory.NewExecutor(), domain)
}
