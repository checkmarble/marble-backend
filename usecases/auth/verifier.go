package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/golang-jwt/jwt/v4"
)

type TokenProvider int

const (
	TokenProviderFirebase TokenProvider = iota
	TokenProviderOidc
)

func ParseTokenProvider(prv string) TokenProvider {
	switch prv {
	case "firebase":
		return TokenProviderFirebase
	case "oidc":
		return TokenProviderOidc
	default:
		return TokenProviderFirebase
	}
}

type TokenProviders []TokenProvider

func ParseTokenProviders(raw string) (TokenProviders, error) {
	if raw == "" {
		raw = TokenProviderFirebase.String()
	}

	providers := make(TokenProviders, 0, 2)
	seen := make(map[TokenProvider]struct{})
	for _, value := range strings.Split(raw, ",") {
		var provider TokenProvider

		switch value = strings.TrimSpace(value); value {
		case TokenProviderFirebase.String():
			provider = TokenProviderFirebase
		case TokenProviderOidc.String():
			provider = TokenProviderOidc
		default:
			return nil, errors.Newf("unsupported token provider: %s", value)
		}

		if _, ok := seen[provider]; ok {
			return nil, errors.Newf("token provider %s is configured more than once", provider)
		}

		seen[provider] = struct{}{}
		providers = append(providers, provider)
	}

	return providers, nil
}

func (providers TokenProviders) Has(provider TokenProvider) bool {
	for _, configuredProvider := range providers {
		if configuredProvider == provider {
			return true
		}
	}

	return false
}

func (providers TokenProviders) Strings() []string {
	values := make([]string, 0, len(providers))
	for _, provider := range providers {
		values = append(values, provider.String())
	}

	return values
}

func (providers TokenProviders) Without(provider TokenProvider) TokenProviders {
	filtered := make(TokenProviders, 0, len(providers))
	for _, configuredProvider := range providers {
		if configuredProvider != provider {
			filtered = append(filtered, configuredProvider)
		}
	}

	return filtered
}

func (p TokenProvider) String() string {
	switch p {
	case TokenProviderFirebase:
		return "firebase"
	case TokenProviderOidc:
		return "oidc"
	default:
		return "firebase"
	}
}

type BaseClaims struct {
	Issuer string `json:"iss"` //nolint:tagliatelle
}

func (BaseClaims) Valid() error {
	return nil
}

type Verifier interface {
	Verify(ctx context.Context, creds Credentials) (models.IntoCredentials, models.IdentityClaims, error)
}

type MarbleVerifier struct {
	verifiers      map[string]idp.TokenRepository
	repository     marbleRepository
	oidcIssuer     string
	allowedDomains []string
}

func NewVerifier(verifiers map[string]idp.TokenRepository, repository marbleRepository, oidcIssuer string, allowedDomains []string) Verifier {
	return MarbleVerifier{
		verifiers:      verifiers,
		repository:     repository,
		oidcIssuer:     oidcIssuer,
		allowedDomains: allowedDomains,
	}
}

func (v MarbleVerifier) Verify(ctx context.Context, creds Credentials) (models.IntoCredentials, models.IdentityClaims, error) {
	switch creds.Type {
	case CredentialsBearer:
		var claims BaseClaims
		if _, _, err := new(jwt.Parser).ParseUnverified(creds.Value, &claims); err != nil {
			return nil, nil, errors.Wrap(err, "could not read token issuer")
		}

		verifier, ok := v.verifiers[claims.Issuer]
		if !ok {
			return nil, nil, errors.Newf("no identity provider is configured for issuer %q", claims.Issuer)
		}

		identity, err := verifier.VerifyToken(ctx, creds.Value, creds.Fallback)
		if err != nil {
			return nil, nil, err
		}

		if claims.Issuer == v.oidcIssuer && len(v.allowedDomains) > 0 {
			domainIsAllowed := false

			for _, domain := range v.allowedDomains {
				if strings.HasSuffix(identity.GetEmail(), domain) {
					domainIsAllowed = true
				}
			}
			if !domainIsAllowed {
				return nil, nil, errors.Newf("oidc user %s cannot authenticate because they are not in the allowed domain list", identity.GetEmail())
			}
		}

		user, err := v.repository.ResolveUserIdentity(ctx, identity.GetIssuer(), identity.GetSubject(), identity.GetEmail())
		if errors.Is(err, models.NotFoundError) {
			return nil, nil, models.ErrUnknownUser
		} else if err != nil {
			return nil, nil, errors.Wrap(err, "repository.ResolveUserIdentity")
		}
		if identity.GetEmail() != "" && identity.GetEmail() != user.Email {
			utils.LoggerFromContext(ctx).InfoContext(ctx, "identity email mismatch",
				"issuer", identity.GetIssuer(),
				"user_id", user.UserId,
			)
		}

		if p := identity.GetProfile(); p != nil {
			user, err = v.repository.UpdateUserProfileFromClaims(ctx, user, *p)
			if err != nil {
				utils.LoggerFromContext(ctx).WarnContext(ctx,
					"could not update user's name", "error", err.Error())

				return user, identity, nil
			}
		}

		return user, identity, nil

	case CredentialsApiKey:
		hash := sha256.Sum256([]byte(creds.Value))

		apiKey, err := v.repository.GetApiKeyByHash(ctx, hash[:])
		if err != nil {
			return nil, nil, fmt.Errorf("getter.GetApiKeyByHash error: %w", err)
		}

		organization, err := v.repository.GetOrganizationByID(ctx, apiKey.OrganizationId)
		if err != nil {
			return nil, nil, fmt.Errorf("getter.GetOrganizationByID error: %w", err)
		}

		apiKey.DisplayString = fmt.Sprintf("Api key %s*** of %s", apiKey.Prefix, organization.Name)

		return apiKey, models.ApiKeyIdentity{}, nil
	}

	return nil, nil, errors.New("unknown credentials kind")
}
