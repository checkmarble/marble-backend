package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type MarbleTokenUseCase struct {
	transactionFactory      repositories.TransactionFactory
	marbleJwtRepository     repositories.MarbleJwtRepository
	firebaseTokenRepository repositories.FireBaseTokenRepository
	userRepository          repositories.UserRepository
	apiKeyRepository        repositories.ApiKeyRepository
	organizationRepository  repositories.OrganizationRepository
	tokenLifetimeMinute     int
	context                 context.Context
}

func (usecase *MarbleTokenUseCase) encodeMarbleToken(creds models.Credentials) (string, time.Time, error) {
	expirationTime := time.Now().Add(time.Duration(usecase.tokenLifetimeMinute) * time.Minute)

	token, err := usecase.marbleJwtRepository.EncodeMarbleToken(expirationTime, creds)
	return token, expirationTime, err
}

func (usecase *MarbleTokenUseCase) adaptCredentialFromApiKey(key string) (models.Credentials, error) {

	apiKey, err := usecase.apiKeyRepository.GetApiKeyByKey(nil, key)
	if err != nil {
		return models.Credentials{}, err
	}

	// Build a token name from the organization name because
	// We don't want to log the apiKey itself.
	apiKeyName, err := usecase.makeTokenName(apiKey.OrganizationId)
	if err != nil {
		return models.Credentials{}, err
	}
	return models.NewCredentialWithApiKey(apiKey.OrganizationId, apiKey.Role, apiKeyName), nil
}

func (usecase *MarbleTokenUseCase) makeTokenName(organizationId string) (string, error) {
	organizationName, err := usecase.organizationRepository.GetOrganizationById(nil, organizationId)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("ApiKey Of %s", organizationName.Name), nil
}

func (usecase *MarbleTokenUseCase) NewMarbleToken(apiKey string, firebaseToken string) (string, time.Time, error) {
	if apiKey != "" {
		credentials, err := usecase.adaptCredentialFromApiKey(apiKey)
		if err != nil {
			return "", time.Time{}, err
		}

		return usecase.encodeMarbleToken(credentials)
	}

	if firebaseToken != "" {
		identity, err := usecase.firebaseTokenRepository.VerifyFirebaseToken(usecase.context, firebaseToken)

		if err != nil {
			return "", time.Time{}, fmt.Errorf("firebase TokenID verification fail: %w", err)
		}

		user, err := repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.User, error) {

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
				return usecase.userRepository.UserByID(tx, user.UserId)
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
func (usecase *MarbleTokenUseCase) ValidateCredentials(marbleToken string, apiKey string) (models.Credentials, error) {
	if apiKey != "" {
		return usecase.adaptCredentialFromApiKey(apiKey)
	}

	if marbleToken != "" {
		return usecase.marbleJwtRepository.ValidateMarbleToken(marbleToken)
	}

	return models.Credentials{}, fmt.Errorf("marble Access Token or X-API-Key is missing: %w", models.UnAuthorizedError)
}
