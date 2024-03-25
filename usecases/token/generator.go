package token

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
	EncodeMarbleToken(expirationTime time.Time, creds models.Credentials) (string, error)
}

type firebaseTokenVerifier interface {
	VerifyFirebaseToken(ctx context.Context, firebaseToken string) (models.FirebaseIdentity, error)
}

type Generator struct {
	repository    marbleRepository
	encoder       encoder
	verifier      firebaseTokenVerifier
	clock         clock.Clock
	tokenLifetime time.Duration
}

func (g *Generator) encodeToken(credentials models.Credentials) (string, time.Time, models.Credentials, error) {
	expirationTime := g.clock.Now().Add(g.tokenLifetime)

	token, err := g.encoder.EncodeMarbleToken(expirationTime, credentials)
	if err != nil {
		return "", time.Time{}, models.Credentials{},
			fmt.Errorf("encoder.EncodeMarbleToken error: %w", err)
	}
	return token, expirationTime, credentials, nil
}

func (g *Generator) FromAPIKey(ctx context.Context, apiKey string) (string, time.Time, models.Credentials, error) {
	hashArr := sha256.Sum256([]byte(apiKey))
	hash := hashArr[:]
	key, err := g.repository.GetApiKeyByHash(ctx, hash)
	if err != nil {
		return "", time.Time{}, models.Credentials{},
			fmt.Errorf("GetApiKeyByHash error: %w", err)
	}

	organization, err := g.repository.GetOrganizationByID(ctx, key.OrganizationId)
	if err != nil {
		return "", time.Time{}, models.Credentials{},
			fmt.Errorf("GetOrganizationByID error: %w", err)
	}

	name := fmt.Sprintf("Api key %s*** of %s", key.Prefix, organization.Name)
	credentials := models.NewCredentialWithApiKey(key.OrganizationId, key.PartnerId, key.Role, name)
	return g.encodeToken(credentials)
}

func (g *Generator) fromFirebaseToken(ctx context.Context, firebaseToken string) (string, time.Time, models.Credentials, error) {
	identity, err := g.verifier.VerifyFirebaseToken(ctx, firebaseToken)
	if err != nil {
		return "", time.Time{}, models.Credentials{},
			fmt.Errorf("verifier.VerifyFirebaseToken error: %w", err)
	}

	user, err := g.repository.UserByEmail(ctx, identity.Email)
	if errors.Is(err, models.NotFoundError) {
		return "", time.Time{}, models.Credentials{},
			fmt.Errorf("%w: %w", models.ErrUnknownUser, err)
	} else if err != nil {
		return "", time.Time{}, models.Credentials{},
			fmt.Errorf("repository.UserByEmail error: %w", err)
	}

	credentials := models.NewCredentialWithUser(user)
	return g.encodeToken(credentials)
}

func (g *Generator) GenerateToken(ctx context.Context, key string, firebaseToken string) (string, time.Time, error) {
	// segment analytics events only for login by an end user with firebase
	if key != "" {
		token, expirationTime, _, err := g.FromAPIKey(ctx, key)
		return token, expirationTime, err
	}

	token, expirationTime, credentials, err := g.fromFirebaseToken(ctx, firebaseToken)
	if err != nil {
		return "", time.Time{}, err
	}

	if credentials.Role != models.MARBLE_ADMIN {
		organization, err := g.repository.GetOrganizationByID(ctx, credentials.OrganizationId)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("GetOrganizationByID error: %w", err)
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

	return token, expirationTime, nil
}

func NewGenerator(repository marbleRepository, encoder encoder, verifier firebaseTokenVerifier, tokenLifetime int) *Generator {
	return &Generator{
		repository:    repository,
		encoder:       encoder,
		verifier:      verifier,
		clock:         clock.New(),
		tokenLifetime: time.Duration(tokenLifetime) * time.Minute,
	}
}
