package dbmodels

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdaptTenant(t *testing.T) {
	id := uuid.New()
	got, err := AdaptTenant(DBTenant{Id: id, Name: "Tenant"})
	require.NoError(t, err)
	require.Equal(t, models.Tenant{Id: id, Name: "Tenant"}, got)
}

func TestAdaptTenantWithoutOIDCConfiguration(t *testing.T) {
	id := uuid.New()
	got, err := AdaptTenantWithOIDCConfiguration(DBTenantWithOIDCConfiguration{DBTenant: DBTenant{Id: id, Name: "Tenant"}})
	require.NoError(t, err)
	require.Equal(t, models.Tenant{Id: id, Name: "Tenant"}, got)
}

func TestAdaptTenantWithOIDCConfiguration(t *testing.T) {
	id := uuid.New()
	issuer, clientId, emailClaim, ssoEnabled := "https://id.example.test", "client", "email", true
	got, err := AdaptTenantWithOIDCConfiguration(DBTenantWithOIDCConfiguration{
		DBTenant:        DBTenant{Id: id, Name: "Tenant"},
		OIDCIssuer:      &issuer,
		OIDCClientId:    &clientId,
		OIDCScopes:      []string{"openid", "email"},
		OIDCExtraParams: []byte(`{"prompt":"login"}`),
		OIDCEmailClaim:  &emailClaim,
		OIDCSsoEnabled:  &ssoEnabled,
	})
	require.NoError(t, err)
	require.Equal(t, models.Tenant{
		Id:   id,
		Name: "Tenant",
		OIDCConfiguration: &models.TenantOIDCConfiguration{
			TenantId:    id,
			Issuer:      issuer,
			ClientId:    clientId,
			Scopes:      []string{"openid", "email"},
			ExtraParams: map[string]string{"prompt": "login"},
			EmailClaim:  emailClaim,
			SsoEnabled:  true,
		},
	}, got)
}
