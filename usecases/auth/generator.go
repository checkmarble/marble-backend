package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/usecases/tracking"
)

type marbleRepository interface {
	GetApiKeyByHash(ctx context.Context, hash []byte) (models.ApiKey, error)
	GetOrganizationByID(ctx context.Context, organizationID string) (models.Organization, error)
	UserByEmail(ctx context.Context, email string) (models.User, error)
}

type encoder interface {
	EncodeMarbleToken(issuer string, expirationTime time.Time, creds models.Credentials) (string, error)
}

type TokenGenerator interface {
	GenerateToken(ctx context.Context, creds Credentials, claims models.IdentityClaims) (Token, error)
}

type Token struct {
	Credentials models.Credentials
	Value       string
	Expiration  time.Time
}

type MarbleTokenGenerator struct {
	repository marbleRepository

	clock         clock.Clock
	tokenLifetime time.Duration
	encoder       encoder
}

func NewGenerator(repository marbleRepository, encoder encoder, lifetime time.Duration, clock clock.Clock) TokenGenerator {
	return MarbleTokenGenerator{
		repository:    repository,
		encoder:       encoder,
		tokenLifetime: lifetime,
		clock:         clock,
	}
}

func (g MarbleTokenGenerator) GenerateToken(ctx context.Context, creds Credentials, claims models.IdentityClaims) (Token, error) {
	expirationTime := g.clock.Now().Add(g.tokenLifetime)

	var credentials models.Credentials

	switch creds.Type {
	case CredentialsBearer:
		user, err := g.repository.UserByEmail(ctx, claims.GetEmail())
		if errors.Is(err, models.NotFoundError) {
			return Token{}, fmt.Errorf("%w: %w", models.ErrUnknownUser, err)
		} else if err != nil {
			return Token{}, fmt.Errorf("repository.UserByEmail error: %w", err)
		}

		credentials = models.NewCredentialWithUser(user)

		if credentials.Role != models.MARBLE_ADMIN {
			organization, err := g.repository.GetOrganizationByID(ctx, credentials.OrganizationId)
			if err != nil {
				return Token{}, fmt.Errorf("GetOrganizationByID error: %w", err)
			}

			tracking.Identify(ctx, credentials.ActorIdentity.UserId, map[string]any{
				"email": credentials.ActorIdentity.Email,
			})
			tracking.Group(ctx, credentials.ActorIdentity.UserId, credentials.OrganizationId, map[string]any{
				"name": organization.Name,
			})
			tracking.TrackEventWithUserId(ctx, models.AnalyticsTokenCreated,
				credentials.ActorIdentity.UserId, map[string]any{
					"organization_id": credentials.OrganizationId,
				})
		}

	case CredentialsApiKey:
		hash := sha256.Sum256([]byte(creds.Value))

		apiKey, err := g.repository.GetApiKeyByHash(ctx, hash[:])
		if err != nil {
			return Token{}, fmt.Errorf("getter.GetApiKeyByHash error: %w", err)
		}

		organization, err := g.repository.GetOrganizationByID(ctx, apiKey.OrganizationId)
		if err != nil {
			return Token{}, fmt.Errorf("getter.GetOrganizationByID error: %w", err)
		}

		name := fmt.Sprintf("Api key %s*** of %s", apiKey.Prefix, organization.Name)
		credentials = models.NewCredentialWithApiKey(apiKey, name)
	}

	token, err := g.encoder.EncodeMarbleToken(claims.GetIssuer(), expirationTime, credentials)
	if err != nil {
		return Token{}, fmt.Errorf("encoder.EncodeMarbleToken error: %w", err)
	}

	return Token{credentials, token, expirationTime}, nil
}
