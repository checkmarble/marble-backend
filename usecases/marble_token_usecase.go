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
}

const TOKEN_LIFETIME_MINUTES = 1

func (usecase *MarbleTokenUseCase) encodeMarbleToken(creds Credentials) (string, time.Time) {
	expirationTime := time.Now().Add(time.Duration(TOKEN_LIFETIME_MINUTES) * time.Minute)

	return usecase.marbleJwtRepository.EncodeMarbleToken(expirationTime, creds), expirationTime
}

func (usecase *MarbleTokenUseCase) NewMarbleToken(ctx context.Context, apiKey string, firebaseIdToken string) (string, *time.Time, error) {
	if apiKey != "" {
		orgID, err := usecase.apiKeyRepository.GetOrganizationIDFromApiKey(ctx, apiKey)
		if err != nil {
			return "", nil, err
		}
		token, time := usecase.encodeMarbleToken(Credentials{OrganizationId: orgID, Role: API_KEY})
		return token, &time, nil
	}

	if firebaseIdToken != "" {
		identity, err := usecase.firebaseTokenRepository.VerifyFirebaseIDToken(ctx, firebaseIdToken)

		if err != nil {
			return "", nil, fmt.Errorf("Firebase TokenID verification fail: %w", UnAuthorizedError)
		}

		user := usecase.userRepository.UserByFirebaseUid(identity.FirebaseUid)
		if user == nil {
			// first connection
			user = usecase.userRepository.UserByEmail(identity.Email)
			if user == nil {
				return "", nil, fmt.Errorf("Unknown user %s: %w", identity.Email, ForbiddenError)
			}
			// store firebase Id
			if err := usecase.userRepository.UpdateFirebaseId(user.UserId, identity.FirebaseUid); err != nil {
				return "", nil, err
			}
		}
		token, time := usecase.encodeMarbleToken(Credentials{OrganizationId: user.OrganizationId, Role: user.Role})
		return token, &time, nil
	}

	return "", nil, fmt.Errorf("API key or Firebase JWT token required: %w", BadParameterError)

}

func (usecase *MarbleTokenUseCase) ValidateMarbleToken(marbleToken string) (Credentials, error) {

	return usecase.marbleJwtRepository.ValidateMarbleToken(marbleToken)
}
