package usecases

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/require"
)

func TestOIDCExchangeUsesTenantHTTPClient(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token","id_token":"id","token_type":"bearer"}`))
	}))
	defer server.Close()
	provider := (&oidc.ProviderConfig{IssuerURL: server.URL, TokenURL: server.URL}).NewProvider(context.Background())
	config := infra.OidcConfig{Provider: provider, ClientId: "client", ClientSecret: "secret", HTTPClient: server.Client()}
	for _, grant := range []string{"authorization_code", "refresh_token"} {
		t.Run(grant, func(t *testing.T) {
			values := url.Values{"grant_type": {grant}, "code": {"code"}, "refresh_token": {"refresh"}}
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			_, err := (OidcUsecase{}).ExchangeToken(t.Context(), config, req)
			require.NoError(t, err)
		})
	}
}
