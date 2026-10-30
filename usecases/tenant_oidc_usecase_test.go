package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
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
