package idp

import (
	"context"
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/coreos/go-oidc/v3/oidc"
)

type oidcTokenVerifier interface {
	Verify(ctx context.Context, idToken string) (*oidc.IDToken, error)
}

type OidcClient struct {
	issuer   string
	verifier oidcTokenVerifier
}

func NewOidcClient(issuer string, verifier *oidc.IDTokenVerifier) *OidcClient {
	return &OidcClient{
		issuer:   issuer,
		verifier: verifier,
	}
}

func (c *OidcClient) VerifyToken(ctx context.Context, idToken string) (models.IdentityClaims, error) {
	token, err := c.verifier.Verify(ctx, idToken)
	if err != nil {
		return models.OidcIdentity{}, err
	}

	var claims models.OidcIdentity

	if err := token.Claims(&claims); err != nil {
		return models.OidcIdentity{}, err
	}
	if claims.Email == "" {
		return models.OidcIdentity{}, errors.New("oidc claims do not contain the principal's email")
	}

	return claims, nil
}

func (c *OidcClient) Issuer() string {
	return c.issuer
}
