package api

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdaptTenantOIDCDiscoveryDoesNotExposeClientSecret(t *testing.T) {
	response := adaptTenantOIDCDiscovery(models.TenantOIDCConfiguration{
		TenantId: uuid.New(), Issuer: "https://id.example.test", ClientId: "client", ClientSecret: "must-not-be-exposed",
	}, "https://app.example.test")

	body, err := json.Marshal(response)

	require.NoError(t, err)
	require.NotContains(t, string(body), "must-not-be-exposed")
	require.Contains(t, string(body), `"redirect_uri":"https://app.example.test/oidc/callback"`)
}
