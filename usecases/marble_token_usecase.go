package usecases

import (
	"context"
	"fmt"
	. "marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"time"
)

type MarbleTokenUseCase struct {
	marbleJwtRepository     repositories.MarbleJwtRepository
	firebaseTokenRepository repositories.FireBaseTokenRepository
	userRepository          repositories.UserRepository
	apiKeyRepository        repositories.ApiKeyRepository
	tokenLifetimeMinute     int
}

func (usecase *MarbleTokenUseCase) encodeMarbleToken(creds Credentials) (string, time.Time, error) {
	expirationTime := time.Now().Add(time.Duration(usecase.tokenLifetimeMinute) * time.Minute)

	token, err := usecase.marbleJwtRepository.EncodeMarbleToken(expirationTime, creds)
	return token, expirationTime, err
}

func (usecase *MarbleTokenUseCase) adaptCredentialFromApiKey(ctx context.Context, apiKey string) (Credentials, error) {
	// Useful to test api as a marble-admin
	// if apiKey == "marble-admin" {
	// 	return Credentials{OrganizationId: "", Role: MARBLE_ADMIN}, nil
	// }
	orgID, err := usecase.apiKeyRepository.GetOrganizationIDFromApiKey(ctx, apiKey)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{OrganizationId: orgID, Role: API_CLIENT}, nil
}

func (usecase *MarbleTokenUseCase) NewMarbleToken(ctx context.Context, apiKey string, firebaseIdToken string) (string, time.Time, error) {
	if apiKey != "" {
		credentials, err := usecase.adaptCredentialFromApiKey(ctx, apiKey)
		if err != nil {
			return "", time.Time{}, err
		}

		return usecase.encodeMarbleToken(credentials)
	}

	if firebaseIdToken != "" {
		identity, err := usecase.firebaseTokenRepository.VerifyFirebaseIDToken(ctx, firebaseIdToken)

		if err != nil {
			return "", time.Time{}, fmt.Errorf("Firebase TokenID verification fail: %w", err)
		}

		user := usecase.userRepository.UserByFirebaseUid(identity.FirebaseUid)
		if user == nil {
			// first connection
			user = usecase.userRepository.UserByEmail(identity.Email)
			if user == nil {
				return "", time.Time{}, fmt.Errorf("Unknown user %s: %w", identity.Email, NotFoundError)
			}
			// store firebase Id
			if err := usecase.userRepository.UpdateFirebaseId(user.UserId, identity.FirebaseUid); err != nil {
				return "", time.Time{}, err
			}
		}
		return usecase.encodeMarbleToken(Credentials{OrganizationId: user.OrganizationId, Role: user.Role})
	}

	return "", time.Time{}, fmt.Errorf("API key or Firebase JWT token required: %w", UnAuthorizedError)
}

func (usecase *MarbleTokenUseCase) ValidateCredentials(ctx context.Context, marbleToken string, apiKey string) (Credentials, error) {
	if apiKey != "" {
		return usecase.adaptCredentialFromApiKey(ctx, apiKey)
	}

	if marbleToken != "" {
		return usecase.marbleJwtRepository.ValidateMarbleToken(marbleToken)
	}

	return Credentials{}, fmt.Errorf("Marble Access Token or X-API-Key is missing: %w", UnAuthorizedError)
}
