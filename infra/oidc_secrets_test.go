package infra

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitializeOidcSecretsKeeper(t *testing.T) {
	key := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	t.Setenv(OidcSecretsKeyEnv, key)

	keeper, err := InitializeOidcSecretsKeeper(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, keeper.Close()) })

	ciphertext, err := keeper.Encrypt(t.Context(), []byte("client secret"))
	require.NoError(t, err)
	require.NotContains(t, string(ciphertext), "client secret")
	plaintext, err := keeper.Decrypt(t.Context(), ciphertext)
	require.NoError(t, err)
	require.Equal(t, []byte("client secret"), plaintext)
}

func TestInitializeOidcSecretsKeeperRequiresKey(t *testing.T) {
	t.Setenv(OidcSecretsKeyEnv, "")
	_, err := InitializeOidcSecretsKeeper(t.Context())
	require.ErrorContains(t, err, OidcSecretsKeyEnv)
}

func TestInitializeOidcSecretsKeeperRejectsInvalidKey(t *testing.T) {
	for _, key := range []string{"not base64", "dG9vLXNob3J0"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(OidcSecretsKeyEnv, key)
			_, err := InitializeOidcSecretsKeeper(t.Context())
			require.ErrorContains(t, err, "decoding OIDC secrets key")
		})
	}
}
