package infra

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/sync/singleflight"
)

type OidcConfig struct {
	Issuer       string
	ClientId     string
	ClientSecret string
	RedirectUri  string
	Scopes       []string
	ExtraParams  map[string]string
	EmailClaim   string

	AllowedDomains []string

	Provider   *oidc.Provider
	Verifier   *oidc.IDTokenVerifier
	HTTPClient *http.Client
}

type TenantOIDCRegistry interface {
	Get(ctx context.Context, tenantId uuid.UUID) (OidcConfig, error)
}

type TenantOIDCConfigurationLoader func(context.Context, uuid.UUID) (models.TenantOIDCConfiguration, error)

type tenantOIDCRegistry struct {
	load        TenantOIDCConfigurationLoader
	redirectURI string
	providers   *expirable.LRU[string, *oidc.Provider]
	loadGroup   singleflight.Group
	allowHTTP   bool
	client      *http.Client
}

const tenantOIDCProviderTimeout = 5 * time.Second

func NewTenantOIDCRegistry(load TenantOIDCConfigurationLoader, redirectURI string, allowHTTP bool) TenantOIDCRegistry {
	return &tenantOIDCRegistry{
		load:        load,
		redirectURI: redirectURI,
		providers:   expirable.NewLRU[string, *oidc.Provider](128, nil, 5*time.Minute),
		allowHTTP:   allowHTTP,
		client: &http.Client{Timeout: tenantOIDCProviderTimeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many OIDC redirects")
			}
			return ValidateTenantOIDCURL(req.URL.String(), allowHTTP)
		}},
	}
}

func ValidateTenantOIDCURL(raw string, allowHTTP bool) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) {
		return errors.Wrap(models.BadParameterError, "OIDC URL must use HTTPS")
	}
	return nil
}

func (registry *tenantOIDCRegistry) Get(ctx context.Context, tenantId uuid.UUID) (OidcConfig, error) {
	configuration, err := registry.load(ctx, tenantId)
	if err != nil {
		return OidcConfig{}, err
	}
	if !configuration.SsoEnabled || len(configuration.AllowedDomains) == 0 {
		return OidcConfig{}, errors.Wrap(models.NotFoundError, "OIDC is not available for this tenant")
	}
	provider, err := registry.provider(ctx, configuration.Issuer)
	if err != nil {
		return OidcConfig{}, err
	}
	return OidcConfig{
		Issuer:         configuration.Issuer,
		ClientId:       configuration.ClientId,
		ClientSecret:   configuration.ClientSecret,
		RedirectUri:    registry.redirectURI,
		Scopes:         configuration.Scopes,
		ExtraParams:    configuration.ExtraParams,
		EmailClaim:     configuration.EmailClaim,
		AllowedDomains: configuration.AllowedDomains,
		Provider:       provider,
		Verifier:       provider.Verifier(&oidc.Config{ClientID: configuration.ClientId}),
		HTTPClient:     registry.client,
	}, nil
}

func (registry *tenantOIDCRegistry) provider(ctx context.Context, issuer string) (*oidc.Provider, error) {
	if err := ValidateTenantOIDCURL(issuer, registry.allowHTTP); err != nil {
		return nil, err
	}
	if provider, ok := registry.providers.Get(issuer); ok {
		return provider, nil
	}

	result := registry.loadGroup.DoChan(issuer, func() (any, error) {
		if provider, ok := registry.providers.Get(issuer); ok {
			return provider, nil
		}
		loadCtx, cancel := context.WithTimeout(context.Background(), tenantOIDCProviderTimeout)
		defer cancel()
		provider, err := oidc.NewProvider(oidc.ClientContext(loadCtx, registry.client), issuer)
		if err != nil {
			return nil, err
		}
		var endpoints struct {
			AuthorizationEndpoint string `json:"authorization_endpoint"`
			TokenEndpoint         string `json:"token_endpoint"`
			JwksUri               string `json:"jwks_uri"`
		}
		if err := provider.Claims(&endpoints); err != nil {
			return nil, err
		}
		for _, endpoint := range []string{endpoints.AuthorizationEndpoint, endpoints.TokenEndpoint, endpoints.JwksUri} {
			if err := ValidateTenantOIDCURL(endpoint, registry.allowHTTP); err != nil {
				return nil, err
			}
		}
		registry.providers.Add(issuer, provider)
		return provider, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case outcome := <-result:
		if outcome.Err != nil {
			return nil, outcome.Err
		}
		return outcome.Val.(*oidc.Provider), nil
	}
}

func InitializeOidc(ctx context.Context, marbleAppUrl string) (OidcConfig, error) {
	issuer := utils.GetEnv("AUTH_OIDC_ISSUER", "")
	clientId := utils.GetEnv("AUTH_OIDC_CLIENT_ID", "")
	if issuer == "" || clientId == "" {
		return OidcConfig{}, errors.Errorf("AUTH_OIDC_ISSUER and AUTH_OIDC_CLIENT_ID are required")
	}
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
		allowedDomains[idx] = "@" + strings.TrimSpace(strings.ToLower(domain))
	}
	scopes := []string{}
	for _, scope := range strings.Split(utils.GetEnv("AUTH_OIDC_SCOPE", ""), ",") {
		if scope = strings.TrimSpace(scope); scope != "" {
			scopes = append(scopes, scope)
		}
	}

	return OidcConfig{
		Issuer:       issuer,
		ClientId:     clientId,
		ClientSecret: utils.GetEnv("AUTH_OIDC_CLIENT_SECRET", ""),
		Scopes:       scopes,
		RedirectUri:  fmt.Sprintf("%s/oidc/callback", marbleAppUrl),
		ExtraParams:  extraParams,
		EmailClaim:   utils.GetEnv("AUTH_OIDC_EMAIL_CLAIM", ""),

		AllowedDomains: allowedDomains,

		Provider: provider,
		Verifier: provider.Verifier(&oidc.Config{
			ClientID: clientId,
		}),
	}, nil
}
