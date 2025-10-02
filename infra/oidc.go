package infra

import (
	"context"
	"net/url"
	"strings"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/coreos/go-oidc/v3/oidc"
)

type OidcConfig struct {
	Issuer       string
	ClientId     string
	ClientSecret string
	RedirectUri  string
	Scopes       []string
	ExtraParams  map[string]string

	Provider *oidc.Provider
	Verifier *oidc.IDTokenVerifier
}

func InitializeOidc(ctx context.Context) (OidcConfig, error) {
	issuer := utils.GetEnv("AUTH_OIDC_ISSUER", "")
	clientId := utils.GetEnv("AUTH_OIDC_CLIENT_ID", "")
	extraParams := map[string]string{}

	if params, err := url.ParseQuery(utils.GetEnv("AUTH_OIDC_EXTRA_PARAMS", "")); err == nil {
		for k, v := range params {
			if len(v) > 0 {
				extraParams[k] = v[0]
			}
		}
	}

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return OidcConfig{}, err
	}

	return OidcConfig{
		Issuer:       issuer,
		ClientId:     clientId,
		ClientSecret: utils.GetEnv("AUTH_OIDC_CLIENT_SECRET", ""),
		Scopes:       strings.Split(utils.GetEnv("AUTH_OIDC_SCOPE", ""), ","),
		RedirectUri:  utils.GetEnv("AUTH_OIDC_REDIRECT_URI", ""),
		ExtraParams:  extraParams,

		Provider: provider,
		Verifier: provider.Verifier(&oidc.Config{
			ClientID: clientId,
		}),
	}, nil
}
