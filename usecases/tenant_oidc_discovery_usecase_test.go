package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type tenantOIDCDiscoveryRepositoryStub struct {
	domain string
	config models.TenantOIDCConfiguration
}

func (r *tenantOIDCDiscoveryRepositoryStub) DiscoverTenantOIDCConfigurationByDomain(_ context.Context, _ repositories.Executor, domain string) (models.TenantOIDCConfiguration, error) {
	r.domain = domain
	return r.config, nil
}

func TestTenantOIDCDiscoveryUsecaseByDomain(t *testing.T) {
	executorFactory := &mocks.ExecutorFactory{}
	executor := &mocks.Executor{}
	executorFactory.On("NewExecutor").Return(executor)
	repository := &tenantOIDCDiscoveryRepositoryStub{config: models.TenantOIDCConfiguration{TenantId: uuid.New()}}
	usecase := NewTenantOIDCDiscoveryUsecase(executorFactory, repository, true)

	configuration, err := usecase.ByDomain(t.Context(), " Example.COM. ")

	require.NoError(t, err)
	require.Equal(t, "example.com", repository.domain)
	require.Equal(t, repository.config, configuration)
	executorFactory.AssertExpectations(t)
}

func TestTenantOIDCDiscoveryUsecaseDoesNotDiscoverWhenSSODisabled(t *testing.T) {
	usecase := NewTenantOIDCDiscoveryUsecase(&mocks.ExecutorFactory{}, &tenantOIDCDiscoveryRepositoryStub{}, false)

	_, err := usecase.ByDomain(t.Context(), "example.com")

	require.ErrorIs(t, err, models.NotFoundError)
}
