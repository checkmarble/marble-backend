package infra

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTenantOIDCRegistryReloadsTenantPolicyButCachesProvider(t *testing.T) {
	var discoveries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		discoveries.Add(1)
		_, _ = w.Write([]byte(`{"issuer":"` + serverURL(r) + `","authorization_endpoint":"https://id.example.test/authorize","token_endpoint":"https://id.example.test/token","jwks_uri":"https://id.example.test/keys"}`))
	}))
	defer server.Close()

	var loads atomic.Int32
	tenantId := uuid.New()
	registry := NewTenantOIDCRegistry(func(_ context.Context, gotTenantId uuid.UUID) (models.TenantOIDCConfiguration, error) {
		require.Equal(t, tenantId, gotTenantId)
		loads.Add(1)
		return models.TenantOIDCConfiguration{TenantId: tenantId, Issuer: server.URL, ClientId: "client", ClientSecret: "secret", AllowedDomains: []string{"@example.test"}, SsoEnabled: true}, nil
	}, "https://app.example.test/oidc/callback", true)

	first, err := registry.Get(t.Context(), tenantId)
	require.NoError(t, err)
	second, err := registry.Get(t.Context(), tenantId)
	require.NoError(t, err)
	require.Equal(t, first.ClientId, second.ClientId)
	require.EqualValues(t, 2, loads.Load())
	require.EqualValues(t, 1, discoveries.Load())
}

func TestTenantOIDCRegistryDoesNotCacheFailure(t *testing.T) {
	tenantId := uuid.New()
	var loads atomic.Int32
	registry := NewTenantOIDCRegistry(func(_ context.Context, _ uuid.UUID) (models.TenantOIDCConfiguration, error) {
		loads.Add(1)
		return models.TenantOIDCConfiguration{}, errors.New("unavailable")
	}, "https://app.example.test/oidc/callback", true)

	_, err := registry.Get(t.Context(), tenantId)
	require.Error(t, err)
	_, err = registry.Get(t.Context(), tenantId)
	require.Error(t, err)
	require.EqualValues(t, 2, loads.Load())
}

func TestTenantOIDCRegistryRejectsTenantWithoutApprovedDomain(t *testing.T) {
	tenantID := uuid.New()
	registry := NewTenantOIDCRegistry(func(_ context.Context, _ uuid.UUID) (models.TenantOIDCConfiguration, error) {
		return models.TenantOIDCConfiguration{TenantId: tenantID, Issuer: "https://id.example.test", SsoEnabled: true}, nil
	}, "https://app.example.test/oidc/callback", false)

	_, err := registry.Get(t.Context(), tenantID)

	require.ErrorIs(t, err, models.NotFoundError)
}

func TestTenantOIDCRegistryReadsTheCurrentTenantPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"issuer":"` + serverURL(r) + `","authorization_endpoint":"https://id.example.test/authorize","token_endpoint":"https://id.example.test/token","jwks_uri":"https://id.example.test/keys"}`))
	}))
	defer server.Close()

	tenantID := uuid.New()
	var enabled atomic.Bool
	enabled.Store(true)
	registry := NewTenantOIDCRegistry(func(_ context.Context, _ uuid.UUID) (models.TenantOIDCConfiguration, error) {
		return models.TenantOIDCConfiguration{
			TenantId: tenantID, Issuer: server.URL, ClientId: "client", ClientSecret: "secret",
			AllowedDomains: []string{"@example.test"}, SsoEnabled: enabled.Load(),
		}, nil
	}, "https://app.example.test/oidc/callback", true)

	_, err := registry.Get(t.Context(), tenantID)
	require.NoError(t, err)
	enabled.Store(false)
	_, err = registry.Get(t.Context(), tenantID)
	require.ErrorIs(t, err, models.NotFoundError)
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}

func TestTenantOIDCRegistryRejectsInsecureEndpointsAndRedirects(t *testing.T) {
	var reached atomic.Int32
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
	}))
	defer insecure.Close()

	for _, redirect := range []bool{false, true} {
		name := "insecure token endpoint"
		if redirect {
			name = "insecure discovery redirect"
		}
		t.Run(name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if redirect {
					http.Redirect(w, r, insecure.URL, http.StatusFound)
					return
				}
				_, _ = w.Write([]byte(`{"issuer":"https://` + r.Host + `","authorization_endpoint":"https://` + r.Host + `/auth","token_endpoint":"` + insecure.URL + `","jwks_uri":"https://` + r.Host + `/keys"}`))
			}))
			defer server.Close()
			registry := NewTenantOIDCRegistry(func(context.Context, uuid.UUID) (models.TenantOIDCConfiguration, error) {
				return models.TenantOIDCConfiguration{Issuer: server.URL, ClientId: "client", SsoEnabled: true, AllowedDomains: []string{"@example.test"}}, nil
			}, "https://app.example.test/callback", false).(*tenantOIDCRegistry)
			registry.client.Transport = server.Client().Transport
			_, err := registry.Get(t.Context(), uuid.New())
			require.Error(t, err)
			require.Zero(t, reached.Load())
		})
	}
}

func TestValidateTenantOIDCURL(t *testing.T) {
	require.NoError(t, ValidateTenantOIDCURL("https://example.test", false))
	require.Error(t, ValidateTenantOIDCURL("http://example.test", false))
	require.NoError(t, ValidateTenantOIDCURL("http://example.test", true))
	require.Error(t, ValidateTenantOIDCURL("ftp://example.test", true))
}

func TestTenantOIDCClientRejectsTokenRedirectToHTTP(t *testing.T) {
	var reached atomic.Int32
	insecure := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Add(1) }))
	defer insecure.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	registry := NewTenantOIDCRegistry(nil, "", false).(*tenantOIDCRegistry)
	registry.client.Transport = server.Client().Transport
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	req.SetBasicAuth("client", "secret")
	_, err = registry.client.Do(req)
	require.Error(t, err)
	require.Zero(t, reached.Load())
}

func TestTenantOIDCRegistryValidatesUserInfoEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name      string
		endpoint  string
		allowHTTP bool
		wantError bool
	}{
		{name: "optional endpoint"},
		{name: "HTTPS", endpoint: "https://id.example.test/userinfo"},
		{name: "HTTP forbidden", endpoint: "http://id.example.test/userinfo", wantError: true},
		{name: "HTTP in development", endpoint: "http://id.example.test/userinfo", allowHTTP: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"issuer":"https://%s","authorization_endpoint":"https://%s/auth","token_endpoint":"https://%s/token","jwks_uri":"https://%s/keys","userinfo_endpoint":%q}`, r.Host, r.Host, r.Host, r.Host, tt.endpoint)
			}))
			defer server.Close()
			registry := NewTenantOIDCRegistry(func(context.Context, uuid.UUID) (models.TenantOIDCConfiguration, error) {
				return models.TenantOIDCConfiguration{Issuer: server.URL, ClientId: "client", SsoEnabled: true, AllowedDomains: []string{"@example.test"}}, nil
			}, "https://app.example.test/callback", tt.allowHTTP).(*tenantOIDCRegistry)
			registry.client.Transport = server.Client().Transport
			_, err := registry.Get(t.Context(), uuid.New())
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTenantOIDCClientRejectsUserInfoRedirectToHTTP(t *testing.T) {
	var reached atomic.Int32
	insecure := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Add(1) }))
	defer insecure.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL, http.StatusFound)
	}))
	defer server.Close()
	registry := NewTenantOIDCRegistry(nil, "", false).(*tenantOIDCRegistry)
	registry.client.Transport = server.Client().Transport
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer access-token")
	_, err = registry.client.Do(req)
	require.Error(t, err)
	require.Zero(t, reached.Load())
}
