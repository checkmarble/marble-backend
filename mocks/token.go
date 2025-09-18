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

func (e StaticTokenVerifier) Verify(_ context.Context, creds auth.Credentials) (models.IdentityClaims, error) {
	if creds.Value == e.Token {
		return e.Claims, nil
	}

	return nil, errors.New("invalid token")
}
