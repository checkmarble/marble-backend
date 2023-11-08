package token

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/usecases/analytics"
)

type marbleRepository interface {
	GetApiKeyByKey(ctx context.Context, key string) (models.ApiKey, error)
	GetOrganizationByID(ctx context.Context, organizationID string) (models.Organization, error)
	UserByFirebaseUid(ctx context.Context, firebaseUID string) (models.User, error)
	UserByEmail(ctx context.Context, email string) (models.User, error)
	UpdateUserFirebaseUID(ctx context.Context, userID models.UserId, firebaseUID string) error
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
		return "", time.Time{}, models.Credentials{}, fmt.Errorf("encoder.EncodeMarbleToken error: %w", err)
	}
	return token, expirationTime, credentials, nil
}

func (g *Generator) fromAPIKey(ctx context.Context, apiKey string) (string, time.Time, models.Credentials, error) {
	key, err := g.repository.GetApiKeyByKey(ctx, apiKey)
	if err != nil {
		return "", time.Time{}, models.Credentials{}, fmt.Errorf("GetApiKeyByKey error: %w", err)
	}

	organization, err := g.repository.GetOrganizationByID(ctx, key.OrganizationId)
	if err != nil {
		return "", time.Time{}, models.Credentials{}, fmt.Errorf("GetOrganizationByID error: %w", err)
	}

	name := fmt.Sprintf("ApiKey Of %s", organization.Name)
	credentials := models.NewCredentialWithApiKey(key.OrganizationId, key.Role, name)
	return g.encodeToken(credentials)
}

func (g *Generator) fromFirebaseToken(ctx context.Context, firebaseToken string) (string, time.Time, models.Credentials, error) {
	identity, err := g.verifier.VerifyFirebaseToken(ctx, firebaseToken)
	if err != nil {
		return "", time.Time{}, models.Credentials{}, fmt.Errorf("verifier.VerifyFirebaseToken error: %w", err)
	}

	user, err := g.repository.UserByFirebaseUid(ctx, identity.FirebaseUid)
	if err == nil {
		credentials := models.NewCredentialWithUser(user.OrganizationId, user.Role, user.UserId, user.Email)
		return g.encodeToken(credentials)
	}
	if err != nil && !errors.Is(err, models.NotFoundError) {
		return "", time.Time{}, models.Credentials{}, fmt.Errorf("repository.UserByFirebaseUid error: %w", err)
	}

	user, err = g.repository.UserByEmail(ctx, identity.Email)
	if err != nil {
		return "", time.Time{}, models.Credentials{}, fmt.Errorf("repository.UserByFirebaseUid error: %w", err)
	}

	if err := g.repository.UpdateUserFirebaseUID(ctx, user.UserId, identity.FirebaseUid); err != nil {
		return "", time.Time{}, models.Credentials{}, fmt.Errorf("repository.UpdateUserFirebaseUID error: %w", err)
	}

	credentials := models.NewCredentialWithUser(user.OrganizationId, user.Role, user.UserId, user.Email)
	return g.encodeToken(credentials)
}

func (g *Generator) GenerateToken(ctx context.Context, key string, firebaseToken string) (string, time.Time, error) {
	// segment analytics events only for login by an end user with firebase
	if key != "" {
		token, expirationTime, _, err := g.fromAPIKey(ctx, key)
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

		analytics.Identify(ctx, credentials.ActorIdentity.UserId, map[string]any{"email": credentials.ActorIdentity.Email})
		analytics.Group(ctx, credentials.ActorIdentity.UserId, credentials.OrganizationId, map[string]any{"name": organization.Name})
		analytics.TrackEventWithUserId(ctx, models.AnalyticsTokenCreated, credentials.ActorIdentity.UserId, map[string]any{"organization_id": credentials.OrganizationId})
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
