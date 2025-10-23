package infra

import (
	"context"
	"fmt"
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

	AllowedDomains []string

	Provider *oidc.Provider
	Verifier *oidc.IDTokenVerifier
}

func InitializeOidc(ctx context.Context, marbleAppUrl string) (OidcConfig, error) {
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

	var allowedDomains []string

	if domains := utils.GetEnv("AUTH_OIDC_ALLOWED_DOMAINS", ""); domains != "" {
		allowedDomains = strings.Split(domains, ",")
	}

	for idx, domain := range allowedDomains {
		allowedDomains[idx] = "@" + domain
	}

	return OidcConfig{
		Issuer:       issuer,
		ClientId:     clientId,
		ClientSecret: utils.GetEnv("AUTH_OIDC_CLIENT_SECRET", ""),
		Scopes:       strings.Split(utils.GetEnv("AUTH_OIDC_SCOPE", ""), ","),
		RedirectUri:  fmt.Sprintf("%s/oidc/callback", marbleAppUrl),
		ExtraParams:  extraParams,

		AllowedDomains: allowedDomains,

		Provider: provider,
		Verifier: provider.Verifier(&oidc.Config{
			ClientID: clientId,
		}),
	}, nil
}
