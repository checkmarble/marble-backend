package idp

import (
	"context"
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type oidcTokenVerifier interface {
	Verify(ctx context.Context, idToken string) (*oidc.IDToken, error)
}

type OidcClient struct {
	issuer   string
	verifier oidcTokenVerifier
	provider *oidc.Provider
}

func NewOidcClient(provider *oidc.Provider, issuer string, verifier *oidc.IDTokenVerifier) *OidcClient {
	return &OidcClient{
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

	if claims.GetEmail() == "" {
		userinfo, err := c.provider.UserInfo(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken}))
		if err != nil {
			return models.OidcIdentity{}, err
		}

		if err := userinfo.Claims(&claims); err != nil {
			return models.OidcIdentity{}, err
		}

		if claims.GetEmail() == "" {
			return models.OidcIdentity{}, errors.New("oidc claims do not contain the principal's email or it is not verified")
		}
	}

	return claims, nil
}

func (c *OidcClient) Issuer() string {
	return c.issuer
}
