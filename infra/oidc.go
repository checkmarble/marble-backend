package infra

import (
	"context"
	"net/url"
	"strings"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/coreos/go-oidc/v3/oidc"
)

type OidcConfig struct {
	Issuer      string
	ClientId    string
	RedirectUri string
	Scopes      []string
	ExtraParams map[string]string

	Provider *oidc.Provider
	Verifier *oidc.IDTokenVerifier
}

func InitializeOidc(ctx context.Context) (OidcConfig, error) {
	extraParams := map[string]string{}

	if params, err := url.ParseQuery(utils.GetEnv("AUTH_OIDC_EXTRA_PARAMS", "")); err == nil {
		for k, v := range params {
			if len(v) > 0 {
				extraParams[k] = v[0]
			}
		}
	}

	cfg := OidcConfig{
		Issuer:      utils.GetEnv("AUTH_OIDC_ISSUER", ""),
		ClientId:    utils.GetEnv("AUTH_OIDC_CLIENT_ID", ""),
		Scopes:      strings.Split(utils.GetEnv("AUTH_OIDC_SCOPE", ""), ","),
		ExtraParams: extraParams,
	}

	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return OidcConfig{}, err
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientId,
	})

	cfg.Provider = provider
	cfg.Verifier = verifier

	return cfg, nil
}
