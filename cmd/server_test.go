package cmd

import (
	"testing"

	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/stretchr/testify/require"
)

func TestConfiguredAuthProvidersIgnoresOIDCSettingsWhenFirebaseSelected(t *testing.T) {
	t.Setenv("AUTH_PROVIDER", "firebase")
	t.Setenv("AUTH_OIDC_ISSUER", "https://id.example.test")
	t.Setenv("AUTH_OIDC_CLIENT_ID", "client")

	providers, err := configuredAuthProviders()

	require.NoError(t, err)
	require.Equal(t, auth.TokenProviders{auth.TokenProviderFirebase}, providers)
}
