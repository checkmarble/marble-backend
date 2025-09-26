package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/utils"
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
	Verify(ctx context.Context, creds Credentials) (models.IntoCredentials, models.IdentityClaims, error)
}

type MarbleVerifier struct {
	flavor     TokenProvider
	verifier   idp.TokenRepository
	repository marbleRepository
}

func NewVerifier(flavor TokenProvider, verifier idp.TokenRepository, repository marbleRepository) Verifier {
	return MarbleVerifier{flavor: flavor, verifier: verifier, repository: repository}
}

func (v MarbleVerifier) Verify(ctx context.Context, creds Credentials) (models.IntoCredentials, models.IdentityClaims, error) {
	switch creds.Type {
	case CredentialsBearer:
		identity, err := v.verifier.VerifyToken(ctx, creds.Value)
		if err != nil {
			return nil, nil, err
		}

		user, err := v.repository.UserByEmail(ctx, identity.GetEmail())
		if errors.Is(err, models.NotFoundError) {
			return nil, nil, fmt.Errorf("%w: %w", models.ErrUnknownUser, err)
		} else if err != nil {
			return nil, nil, fmt.Errorf("repository.UserByEmail error: %w", err)
		}

		if fn, ln, ok := identity.GetName(); ok {
			user, err = v.repository.UpdateUser(ctx, user, fn, ln)
			if err != nil {
				utils.LoggerFromContext(ctx).WarnContext(ctx, "could not update user's name", "error", err.Error())

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
