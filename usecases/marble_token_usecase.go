package usecases

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"time"
)

type MarbleTokenUseCase struct {
	transactionFactory      repositories.TransactionFactory
	marbleJwtRepository     repositories.MarbleJwtRepository
	firebaseTokenRepository repositories.FireBaseTokenRepository
	userRepository          repositories.UserRepository
	apiKeyRepository        repositories.ApiKeyRepository
	organizationRepository  repositories.OrganizationRepository
	tokenLifetimeMinute     int
}

func (usecase *MarbleTokenUseCase) encodeMarbleToken(creds models.Credentials) (string, time.Time, error) {
	expirationTime := time.Now().Add(time.Duration(usecase.tokenLifetimeMinute) * time.Minute)

	token, err := usecase.marbleJwtRepository.EncodeMarbleToken(expirationTime, creds)
	return token, expirationTime, err
}

func (usecase *MarbleTokenUseCase) adaptCredentialFromApiKey(ctx context.Context, apiKey string) (models.Credentials, error) {
	// Useful to test api as a marble-admin
	// if apiKey == "marble-admin" {
	// 	return NewCredentialWithUser("", MARBLE_ADMIN, "", "vivien.miniussi@checkmarble.com"), nil
	// }
	organizationId, err := usecase.apiKeyRepository.GetOrganizationIDFromApiKey(ctx, apiKey)
	if err != nil {
		return models.Credentials{}, err
	}

	// Build a token name from the organization name because
	// We don't want to log the apiKey itself.
	apiKeyName, err := usecase.makeTokenName(ctx, organizationId)
	if err != nil {
		return models.Credentials{}, err
	}
	return models.NewCredentialWithApiKey(organizationId, models.API_CLIENT, apiKeyName), nil
}

func (usecase *MarbleTokenUseCase) makeTokenName(ctx context.Context, organizationId string) (string, error) {
	organizationName, err := usecase.organizationRepository.GetOrganization(ctx, organizationId)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("ApiKey Of %s", organizationName), nil
}

func (usecase *MarbleTokenUseCase) NewMarbleToken(ctx context.Context, apiKey string, firebaseToken string) (string, time.Time, error) {
	if apiKey != "" {
		credentials, err := usecase.adaptCredentialFromApiKey(ctx, apiKey)
		if err != nil {
			return "", time.Time{}, err
		}

		return usecase.encodeMarbleToken(credentials)
	}

	if firebaseToken != "" {
		identity, err := usecase.firebaseTokenRepository.VerifyFirebaseToken(ctx, firebaseToken)

		if err != nil {
			return "", time.Time{}, fmt.Errorf("firebase TokenID verification fail: %w", err)
		}

		user, err := repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE, func(tx repositories.Transaction) (models.User, error) {

			user, err := usecase.userRepository.UserByFirebaseUid(tx, identity.FirebaseUid)
			if err != nil {
				return models.User{}, err
			}
			if user != nil {
				return *user, nil
			}

			// first connection
			user, err = usecase.userRepository.UserByEmail(tx, identity.Email)
			if err != nil {
				return models.User{}, err
			}
			if user != nil {
				// store firebase Id
				if err := usecase.userRepository.UpdateFirebaseId(tx, user.UserId, identity.FirebaseUid); err != nil {
					return models.User{}, err
				}
				return usecase.userRepository.UserByUid(tx, user.UserId)
			}

			return models.User{}, fmt.Errorf("unknown user %s: %w", identity.Email, models.NotFoundError)
		})

		if err != nil {
			return "", time.Time{}, err
		}

		return usecase.encodeMarbleToken(models.NewCredentialWithUser(user.OrganizationId, user.Role, user.UserId, user.Email))
	}

	return "", time.Time{}, fmt.Errorf("API key or Firebase JWT token required: %w", models.UnAuthorizedError)
}

// ValidateCredentials returns the credentials associated with the given marbleToken or apiKey
func (usecase *MarbleTokenUseCase) ValidateCredentials(ctx context.Context, marbleToken string, apiKey string) (models.Credentials, error) {
	if apiKey != "" {
		return usecase.adaptCredentialFromApiKey(ctx, apiKey)
	}

	if marbleToken != "" {
		return usecase.marbleJwtRepository.ValidateMarbleToken(marbleToken)
	}

	return models.Credentials{}, fmt.Errorf("marble Access Token or X-API-Key is missing: %w", models.UnAuthorizedError)
}
