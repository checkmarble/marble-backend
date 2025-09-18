package auth

import (
	"context"
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/idp"
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
	jwt.RegisteredClaims

	Issuer string `json:"iss"` //nolint:tagliatelle
}

type Verifier interface {
	Verify(ctx context.Context, creds Credentials) (models.IdentityClaims, error)
}

type MarbleVerifier struct {
	flavor   TokenProvider
	verifier idp.TokenRepository
}

func NewVerifier(flavor TokenProvider, verifier idp.TokenRepository) Verifier {
	return MarbleVerifier{flavor: flavor, verifier: verifier}
}

func (v MarbleVerifier) Verify(ctx context.Context, creds Credentials) (models.IdentityClaims, error) {
	switch creds.Type {
	case CredentialsBearer:
		identity, err := v.verifier.VerifyToken(ctx, creds.Value)
		if err != nil {
			return nil, err
		}

		return identity, nil

	case CredentialsApiKey:
		return models.ApiKeyIdentity{}, nil
	}

	return nil, errors.New("unknown credentials kind")
}
