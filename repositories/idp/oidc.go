package idp

import (
	"context"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type oidcTokenVerifier interface {
	Verify(ctx context.Context, idToken string) (*oidc.IDToken, error)
}

type OidcClient struct {
	config   infra.OidcConfig
	issuer   string
	verifier oidcTokenVerifier
	provider *oidc.Provider
}

func NewOidcClient(cfg infra.OidcConfig, provider *oidc.Provider, issuer string, verifier *oidc.IDTokenVerifier) *OidcClient {
	return &OidcClient{
		config:   cfg,
		issuer:   issuer,
		verifier: verifier,
		provider: provider,
	}
}

func (c *OidcClient) VerifyToken(ctx context.Context, idToken, accessToken string) (models.IdentityClaims, error) {
	token, err := c.verifier.Verify(ctx, idToken)
	if err != nil {
		return models.OidcIdentity{}, err
	}

	var claims models.OidcIdentity

	if err := token.Claims(&claims); err != nil {
		return models.OidcIdentity{}, err
	}

	// If we were configured to lookup the user's email address from another claim,
	// unmarshal the claims into a map and parse it as a string.
	if c.config.EmailClaim != "" {
		claims, err = c.extractCustomEmailClaim(token, claims)
		if err != nil {
			return models.OidcIdentity{}, err
		}
	}

	if claims.GetEmail() == "" {
		userinfo, err := c.provider.UserInfo(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken}))
		if err != nil {
			return models.OidcIdentity{}, err
		}

		if err := userinfo.Claims(&claims); err != nil {
			return models.OidcIdentity{}, err
		}

		if c.config.EmailClaim != "" {
			claims, err = c.extractCustomEmailClaim(userinfo, claims)
			if err != nil {
				return models.OidcIdentity{}, err
			}
		}
	}

	if claims.GetEmail() == "" {
		return models.OidcIdentity{}, errors.New("oidc claims do not contain the principal's email or it is not verified")
	}

	return claims, nil
}

func (c *OidcClient) Issuer() string {
	return c.issuer
}

type claimExtracter interface {
	Claims(any) error
}

func (c *OidcClient) extractCustomEmailClaim(extractor claimExtracter, claims models.OidcIdentity) (models.OidcIdentity, error) {
	var rawClaims map[string]any

	if err := extractor.Claims(&rawClaims); err != nil {
		return models.OidcIdentity{}, err
	}

	value, ok := rawClaims[c.config.EmailClaim]
	if !ok {
		return models.OidcIdentity{}, errors.Newf("oidc claims do not contain the principal's '%s' claim", c.config.EmailClaim)
	}
	email, ok := value.(string)
	if !ok || email == "" {
		return models.OidcIdentity{}, errors.Newf("oidc claims contain '%s' but it is not a string", c.config.EmailClaim)
	}

	claims.Email = email
	claims.SkipEmailVerify = true

	return claims, nil
}
