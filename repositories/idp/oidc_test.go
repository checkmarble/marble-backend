package idp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOIDCClaims struct {
	claims map[string]any
	err    error
}

func (f fakeOIDCClaims) Claims(destination any) error {
	if f.err != nil {
		return f.err
	}
	encoded, err := json.Marshal(f.claims)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, destination)
}

type fakeOIDCTokenVerifier struct {
	claims oidcClaims
	err    error
}

func (f fakeOIDCTokenVerifier) Verify(context.Context, string) (oidcClaims, error) {
	return f.claims, f.err
}

func TestOidcClient_VerifyToken(t *testing.T) {
	t.Run("uses verified ID token claims", func(t *testing.T) {
		client := &OidcClient{
			issuer: "https://issuer.example.test",
			verifier: fakeOIDCTokenVerifier{claims: fakeOIDCClaims{claims: map[string]any{
				"sub": "subject-1", "email": "user@example.test", "email_verified": true,
			}}},
		}

		identity, err := client.VerifyToken(t.Context(), "id-token", "access-token")
		require.NoError(t, err)
		assert.Equal(t, models.OidcIdentity{
			Subject:       "subject-1",
			Email:         "user@example.test",
			EmailVerified: true,
		}, identity)
	})

	t.Run("rejects missing subject", func(t *testing.T) {
		client := &OidcClient{verifier: fakeOIDCTokenVerifier{claims: fakeOIDCClaims{claims: map[string]any{
			"email": "user@example.test", "email_verified": true,
		}}}}

		_, err := client.VerifyToken(t.Context(), "id-token", "access-token")
		assert.ErrorContains(t, err, "subject")
	})

	t.Run("returns verifier errors", func(t *testing.T) {
		client := &OidcClient{verifier: fakeOIDCTokenVerifier{err: assert.AnError}}

		_, err := client.VerifyToken(t.Context(), "id-token", "access-token")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("uses custom email claim without email_verified", func(t *testing.T) {
		client := &OidcClient{
			config: infra.OidcConfig{EmailClaim: "preferred_email"},
			verifier: fakeOIDCTokenVerifier{claims: fakeOIDCClaims{claims: map[string]any{
				"sub": "subject-1", "preferred_email": "user@example.test",
			}}},
		}

		identity, err := client.VerifyToken(t.Context(), "id-token", "access-token")
		require.NoError(t, err)
		assert.Equal(t, "subject-1", identity.GetSubject())
		assert.Equal(t, "user@example.test", identity.GetEmail())
	})

	t.Run("rejects unverified email without userinfo email", func(t *testing.T) {
		provider := newTestOIDCProvider(t, `{"sub":"subject-1"}`)
		client := &OidcClient{
			provider: provider,
			verifier: fakeOIDCTokenVerifier{claims: fakeOIDCClaims{claims: map[string]any{
				"sub": "subject-1", "email": "user@example.test", "email_verified": false,
			}}},
		}

		_, err := client.VerifyToken(t.Context(), "id-token", "access-token")
		assert.ErrorContains(t, err, "email or it is not verified")
	})

	t.Run("rejects userinfo subject mismatch", func(t *testing.T) {
		provider := newTestOIDCProvider(t, `{"sub":"another-subject","email":"user@example.test","email_verified":true}`)
		client := &OidcClient{
			provider: provider,
			verifier: fakeOIDCTokenVerifier{claims: fakeOIDCClaims{claims: map[string]any{
				"sub": "subject-1", "email": "user@example.test", "email_verified": false,
			}}},
		}

		_, err := client.VerifyToken(t.Context(), "id-token", "access-token")
		assert.ErrorContains(t, err, "subject does not match")
	})

	t.Run("uses matching userinfo subject and profile", func(t *testing.T) {
		provider := newTestOIDCProvider(t, `{"sub":"subject-1","email":"user@example.test","email_verified":true,"given_name":"Test","family_name":"User"}`)
		client := &OidcClient{
			provider: provider,
			verifier: fakeOIDCTokenVerifier{claims: fakeOIDCClaims{claims: map[string]any{
				"sub": "subject-1", "email": "user@example.test", "email_verified": false,
			}}},
		}

		identity, err := client.VerifyToken(t.Context(), "id-token", "access-token")
		require.NoError(t, err)
		assert.Equal(t, "subject-1", identity.GetSubject())
		assert.Equal(t, "user@example.test", identity.GetEmail())
		assert.Equal(t, "Test", identity.GetProfile().Firstname)
		assert.Equal(t, "User", identity.GetProfile().Lastname)
	})

	t.Run("preserves ID token profile when userinfo only contains an email", func(t *testing.T) {
		provider := newTestOIDCProvider(t, `{"sub":"subject-1","email":"user@example.test","email_verified":true}`)
		client := &OidcClient{
			provider: provider,
			verifier: fakeOIDCTokenVerifier{claims: fakeOIDCClaims{claims: map[string]any{
				"sub": "subject-1", "email": "user@example.test", "email_verified": false,
				"given_name": "Test", "family_name": "User", "picture": "https://example.test/picture.jpg",
			}}},
		}

		identity, err := client.VerifyToken(t.Context(), "id-token", "access-token")
		require.NoError(t, err)
		assert.Equal(t, &models.IdentityUpdatableClaims{
			Firstname: "Test", Lastname: "User", Picture: "https://example.test/picture.jpg",
		}, identity.GetProfile())
	})
}

func TestOidcClient_extractCustomEmailClaim(t *testing.T) {
	client := &OidcClient{config: infra.OidcConfig{EmailClaim: "preferred_email"}}

	tests := []struct {
		name    string
		claims  map[string]any
		wantErr string
	}{
		{name: "missing claim", claims: map[string]any{}, wantErr: "do not contain"},
		{name: "claim is not a string", claims: map[string]any{"preferred_email": 42}, wantErr: "not a string"},
		{name: "empty claim", claims: map[string]any{"preferred_email": ""}, wantErr: "not a string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.extractCustomEmailClaim(fakeOIDCClaims{claims: tt.claims}, models.OidcIdentity{})
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func newTestOIDCProvider(t *testing.T, userinfo string) *oidc.Provider {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = w.Write([]byte(`{"issuer":"` + server.URL + `","userinfo_endpoint":"` + server.URL + `/userinfo"}`))
		case "/userinfo":
			_, _ = w.Write([]byte(userinfo))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	provider, err := oidc.NewProvider(t.Context(), server.URL)
	require.NoError(t, err)
	return provider
}
