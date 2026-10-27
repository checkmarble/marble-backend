package dto

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdaptTenantOIDCConfigurationDoesNotExposeClientSecret(t *testing.T) {
	response := AdaptTenantOIDCConfiguration(models.TenantOIDCConfiguration{
		TenantId:     uuid.New(),
		Issuer:       "https://id.example.test",
		ClientId:     "client",
		ClientSecret: "must-not-be-exposed",
		SsoEnabled:   true,
	})

	body, err := json.Marshal(response)

	require.NoError(t, err)
	require.NotContains(t, string(body), "must-not-be-exposed")
	require.Contains(t, string(body), `"client_secret_set":true`)
}

func TestAdaptTenantSerializesMissingOIDCConfigurationAsNull(t *testing.T) {
	body, err := json.Marshal(AdaptTenant(models.Tenant{Id: uuid.New(), Name: "Tenant"}))

	require.NoError(t, err)
	require.Contains(t, string(body), `"oidc_configuration":null`)
}

func TestAdaptTenantIncludesOIDCConfiguration(t *testing.T) {
	tenantId := uuid.New()
	got := AdaptTenant(models.Tenant{
		Id:   tenantId,
		Name: "Tenant",
		OIDCConfiguration: &models.TenantOIDCConfiguration{
			TenantId: tenantId, Issuer: "https://id.example.test", ClientId: "client",
			Scopes: []string{"openid"}, EmailClaim: "email", SsoEnabled: true,
		},
	})

	require.Equal(t, &TenantOIDCConfiguration{
		TenantId: tenantId, Issuer: "https://id.example.test", ClientId: "client", ClientSecretSet: true,
		Scopes: []string{"openid"}, EmailClaim: "email", SsoEnabled: true,
	}, got.OIDCConfiguration)
}
