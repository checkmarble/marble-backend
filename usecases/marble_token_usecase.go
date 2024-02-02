package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type MarbleTokenUseCase struct {
	transactionFactory      transaction.TransactionFactory
	marbleJwtRepository     repositories.MarbleJwtRepository
	firebaseTokenRepository repositories.FireBaseTokenRepository
	userRepository          repositories.UserRepository
	apiKeyRepository        interface {
		GetApiKeyByKey(ctx context.Context, tx repositories.Transaction, apiKey string) (models.ApiKey, error)
	}
	organizationRepository repositories.OrganizationRepository
	tokenLifetimeMinute    int
	context                context.Context
}

func (usecase *MarbleTokenUseCase) encodeMarbleToken(creds models.Credentials) (string, time.Time, error) {
	expirationTime := time.Now().Add(time.Duration(usecase.tokenLifetimeMinute) * time.Minute)

	token, err := usecase.marbleJwtRepository.EncodeMarbleToken(expirationTime, creds)
	return token, expirationTime, err
}

func (usecase *MarbleTokenUseCase) adaptCredentialFromApiKey(ctx context.Context, key string) (models.Credentials, error) {

	apiKey, err := usecase.apiKeyRepository.GetApiKeyByKey(ctx, nil, key)
	if err != nil {
		return models.Credentials{}, err
	}

	// Build a token name from the organization name because
	// We don't want to log the apiKey itself.
	apiKeyName, err := usecase.makeTokenName(ctx, apiKey)
	if err != nil {
		return models.Credentials{}, err
	}
	return models.NewCredentialWithApiKey(apiKey.OrganizationId, apiKey.Role, apiKeyName), nil
}

func (usecase *MarbleTokenUseCase) makeTokenName(ctx context.Context, apiKey models.ApiKey) (string, error) {
	organizationName, err := usecase.organizationRepository.GetOrganizationById(ctx, nil, apiKey.OrganizationId)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("ApiKey Of %s: %s", organizationName.Name, apiKey.Description), nil
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
		identity, err := usecase.firebaseTokenRepository.VerifyFirebaseToken(usecase.context, firebaseToken)

		if err != nil {
			return "", time.Time{}, fmt.Errorf("firebase TokenID verification fail: %w", err)
		}

		user, err := transaction.TransactionReturnValue(ctx, usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.User, error) {
			user, err := usecase.userRepository.UserByEmail(ctx, tx, identity.Email)
			if err != nil {
				return models.User{}, err
			}
			if user == nil {
				return models.User{}, fmt.Errorf("unknown user %s: %w", identity.Email, models.NotFoundError)
			}
			return *user, nil
		})

		if err != nil {
			return "", time.Time{}, err
		}

		return usecase.encodeMarbleToken(models.NewCredentialWithUser(user))
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
