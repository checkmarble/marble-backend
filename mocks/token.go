package mocks

import (
	"context"
	"errors"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/auth"
)

type StaticTokenExtractor struct {
	Creds auth.Credentials
}

func NewStaticTokenExtractor(creds auth.Credentials) StaticTokenExtractor {
	return StaticTokenExtractor{Creds: creds}
}

func (e StaticTokenExtractor) Extract(_ *http.Request) (auth.Credentials, error) {
	return e.Creds, nil
}

type StaticTokenVerifier struct {
	Token  string
	Claims models.IdentityClaims
}

func NewStaticTokenVerifier(token string, claims models.IdentityClaims) StaticTokenVerifier {
	return StaticTokenVerifier{Token: token, Claims: claims}
}

func (e StaticTokenVerifier) Verify(_ context.Context, creds auth.Credentials) (models.IntoCredentials, models.IdentityClaims, error) {
	if creds.Value == e.Token {
		return models.User{}, e.Claims, nil
	}

	return nil, nil, errors.New("invalid token")
}

type StaticIdpTokenVerifier struct {
	issuer string
	claims models.IdentityClaims
}

func NewStaticIdpTokenVerifier(issuer string, claims models.IdentityClaims) StaticIdpTokenVerifier {
	return StaticIdpTokenVerifier{
		issuer: issuer,
		claims: claims,
	}
}

func (e StaticIdpTokenVerifier) Issuer() string {
	return e.issuer
}

func (e StaticIdpTokenVerifier) VerifyToken(ctx context.Context, idToken string) (models.IdentityClaims, error) {
	return e.claims, nil
}
