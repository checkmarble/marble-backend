package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTenantDomain(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "normalizes a verified customer domain", input: " Example.COM. ", want: "example.com"},
		{name: "accepts public suffix for staff review", input: "co.uk", want: "co.uk"},
		{name: "accepts public email domain for staff review", input: "gmail.com", want: "gmail.com"},
		{name: "rejects email address", input: "admin@example.com", wantErr: models.BadParameterError},
		{name: "rejects malformed labels", input: "example..com", wantErr: models.BadParameterError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTenantDomain(tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

type tenantOIDCTestRepository struct {
	repositories.TenantOIDCRepository
	current models.TenantOIDCConfiguration
	err     error
	saved   *models.TenantOIDCConfiguration
}

type tenantOIDCReadRepository struct {
	repositories.TenantOIDCRepository
	exec        repositories.Executor
	tenant      uuid.UUID
	config      models.TenantOIDCConfiguration
	domains     []models.TenantDomain
	err         error
	configCalls int
	domainCalls int
}

func (r *tenantOIDCReadRepository) GetTenantOIDCConfiguration(_ context.Context, exec repositories.Executor, tenant uuid.UUID) (models.TenantOIDCConfiguration, error) {
	r.exec, r.tenant = exec, tenant
	r.configCalls++
	return r.config, r.err
}

func (r *tenantOIDCReadRepository) ListTenantDomains(_ context.Context, exec repositories.Executor, tenant uuid.UUID) ([]models.TenantDomain, error) {
	r.exec, r.tenant = exec, tenant
	r.domainCalls++
	return r.domains, r.err
}

func TestTenantOIDCReadsRejectUnauthorizedTenant(t *testing.T) {
	tenant := uuid.New()
	guard := &tenantScopeSecurityMock{t: t, tenant: tenant}
	factory := &mocks.ExecutorFactory{}
	factory.On("NewExecutor").Return(&mocks.Executor{})
	repo := &tenantOIDCReadRepository{}
	uc := NewTenantOIDCUsecase(guard, &mocks.TransactionFactory{}, factory, repo, &mocks.TenantRepository{}, true, false)

	config, err := uc.GetConfiguration(t.Context(), tenant)
	require.ErrorIs(t, err, models.ForbiddenError)
	require.Equal(t, models.TenantOIDCConfiguration{}, config)
	domains, err := uc.ListDomains(t.Context(), tenant)
	require.ErrorIs(t, err, models.ForbiddenError)
	require.Nil(t, domains)
	require.Equal(t, 2, guard.calls)
	require.Zero(t, repo.configCalls)
	require.Zero(t, repo.domainCalls)
	factory.AssertNotCalled(t, "NewExecutor")
}

func TestTenantOIDCReadsAsGlobalAdmin(t *testing.T) {
	tenant := uuid.New()
	creds := models.Credentials{Roles: []models.Role{models.MARBLE_ADMIN}}
	guard := &security.EnforceSecurityTenantImpl{EnforceSecurity: security.NewEnforceSecurity(creds), Credentials: creds}
	exec := &mocks.Executor{}
	factory := &mocks.ExecutorFactory{}
	factory.On("NewExecutor").Return(exec)
	repo := &tenantOIDCReadRepository{
		config:  models.TenantOIDCConfiguration{TenantId: tenant, Issuer: "https://id.example.test"},
		domains: []models.TenantDomain{{TenantId: tenant, Domain: "example.test"}},
	}
	uc := NewTenantOIDCUsecase(guard, &mocks.TransactionFactory{}, factory, repo, &mocks.TenantRepository{}, true, false)

	config, err := uc.GetConfiguration(t.Context(), tenant)
	require.NoError(t, err)
	require.Equal(t, repo.config, config)
	require.Equal(t, tenant, repo.tenant)
	require.Same(t, exec, repo.exec)
	require.Equal(t, 1, repo.configCalls)
	domains, err := uc.ListDomains(t.Context(), tenant)
	require.NoError(t, err)
	require.Equal(t, repo.domains, domains)
	require.Equal(t, tenant, repo.tenant)
	require.Same(t, exec, repo.exec)
	require.Equal(t, 1, repo.domainCalls)

	repo.err = models.NotFoundError
	_, err = uc.GetConfiguration(t.Context(), tenant)
	require.ErrorIs(t, err, models.NotFoundError)
	repo.err = models.ConflictError
	_, err = uc.ListDomains(t.Context(), tenant)
	require.ErrorIs(t, err, models.ConflictError)
}

func (r *tenantOIDCTestRepository) GetTenantOIDCConfigurationForAuthentication(context.Context, repositories.Executor, uuid.UUID) (models.TenantOIDCConfiguration, error) {
	return r.current, r.err
}

func (r *tenantOIDCTestRepository) UpsertTenantOIDCConfiguration(_ context.Context, _ repositories.Executor, config models.TenantOIDCConfiguration) error {
	r.saved = &config
	return nil
}

func TestTenantOIDCSaveConfigurationSecretReuse(t *testing.T) {
	secret := "new-secret"
	for _, tt := range []struct {
		name       string
		issuer     string
		client     string
		secret     *string
		currentErr error
		wantErr    error
		wantSecret string
	}{
		{name: "reuse", issuer: " https://id.example.com ", client: " client ", wantSecret: "old-secret"},
		{name: "changed issuer", issuer: "https://other.example.com", client: "client", wantErr: models.BadParameterError},
		{name: "changed client", issuer: "https://id.example.com", client: "other", wantErr: models.BadParameterError},
		{name: "provided secret", issuer: "https://other.example.com", client: "other", secret: &secret, wantSecret: "new-secret"},
		{name: "new configuration without secret", issuer: "https://id.example.com", client: "client", currentErr: models.NotFoundError, wantErr: models.BadParameterError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.New()
			repo := &tenantOIDCTestRepository{current: models.TenantOIDCConfiguration{Issuer: "https://id.example.com", ClientId: "client", ClientSecret: "old-secret"}, err: tt.currentErr}
			tx := &mocks.TransactionFactory{TxMock: &mocks.Transaction{}}
			tx.On("Transaction", mock.Anything, mock.Anything).Return(nil)
			tenants := &mocks.TenantRepository{}
			tenants.On("LockAndValidateUpdate", mock.Anything, mock.Anything, id).Return(nil)
			uc := NewTenantOIDCUsecase(tenantMergeSecurityMock{}, tx, &mocks.ExecutorFactory{}, repo, tenants, true, false)
			err := uc.SaveConfiguration(t.Context(), id, TenantOIDCConfigurationInput{Issuer: tt.issuer, ClientId: tt.client, ClientSecret: tt.secret})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, repo.saved)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantSecret, repo.saved.ClientSecret)
		})
	}
}

func TestTenantOIDCWritesRejectUnauthorizedTenant(t *testing.T) {
	tenant := uuid.New()
	security := &tenantScopeSecurityMock{t: t, tenant: tenant}
	uc := NewTenantOIDCUsecase(security,
		&mocks.TransactionFactory{}, &mocks.ExecutorFactory{}, &tenantOIDCTestRepository{}, &mocks.TenantRepository{}, true, false)
	ctx := t.Context()
	require.ErrorIs(t, uc.SaveConfiguration(ctx, tenant, TenantOIDCConfigurationInput{}), models.ForbiddenError)
	require.ErrorIs(t, uc.DeleteConfiguration(ctx, tenant), models.ForbiddenError)
	_, err := uc.AddDomain(ctx, tenant, "example.com")
	require.ErrorIs(t, err, models.ForbiddenError)
	require.ErrorIs(t, uc.DeleteDomain(ctx, tenant, uuid.New()), models.ForbiddenError)
	require.Equal(t, 4, security.calls)
}

func TestValidateTenantOIDCConfiguration(t *testing.T) {
	secret := "secret"
	tests := []struct {
		name    string
		input   TenantOIDCConfigurationInput
		wantErr error
	}{
		{name: "accepts valid configuration", input: TenantOIDCConfigurationInput{Issuer: "https://id.example.com", ClientId: "client", ClientSecret: &secret}},
		{name: "rejects relative issuer", input: TenantOIDCConfigurationInput{Issuer: "/issuer", ClientId: "client", ClientSecret: &secret}, wantErr: models.BadParameterError},
		{name: "rejects HTTP issuer", input: TenantOIDCConfigurationInput{Issuer: "http://id.example.com", ClientId: "client", ClientSecret: &secret}, wantErr: models.BadParameterError},
		{name: "rejects missing client ID", input: TenantOIDCConfigurationInput{Issuer: "https://id.example.com", ClientSecret: &secret}, wantErr: models.BadParameterError},
		{name: "rejects empty supplied secret", input: TenantOIDCConfigurationInput{Issuer: "https://id.example.com", ClientId: "client", ClientSecret: new(string)}, wantErr: models.BadParameterError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTenantOIDCConfiguration(tt.input, false)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestTenantOIDCScopes(t *testing.T) {
	require.Equal(t, []string{"openid", "profile", "email"}, tenantOIDCScopes(nil))
	require.Equal(t, []string{"openid"}, tenantOIDCScopes([]string{"openid"}))
}
