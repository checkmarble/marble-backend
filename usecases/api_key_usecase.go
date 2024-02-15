package usecases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type ApiKeyRepository interface {
	GetApiKeyById(ctx context.Context, exec repositories.Executor, apiKeyId string) (models.ApiKey, error)
	ListApiKeys(ctx context.Context, exec repositories.Executor, organizationId string) ([]models.ApiKey, error)
	CreateApiKey(ctx context.Context, exec repositories.Executor, apiKey models.CreateApiKey) error
	SoftDeleteApiKey(ctx context.Context, exec repositories.Executor, apiKeyId string) error
}

type EnforceSecurityApiKey interface {
	ReadApiKey(apiKey models.ApiKey) error
	CreateApiKey(organizationId string) error
	DeleteApiKey(apiKey models.ApiKey) error
}

type ApiKeyUseCase struct {
	transactionFactory      executor_factory.TransactionFactory
	executorFactory         executor_factory.ExecutorFactory
	organizationIdOfContext func() (string, error)
	enforceSecurity         EnforceSecurityApiKey
	apiKeyRepository        ApiKeyRepository
}

func (usecase *ApiKeyUseCase) ListApiKeys(ctx context.Context) ([]models.ApiKey, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return []models.ApiKey{}, err
	}

	apiKeys, err := usecase.apiKeyRepository.ListApiKeys(ctx,
		usecase.executorFactory.NewExecutor(), organizationId)
	if err != nil {
		return []models.ApiKey{}, err
	}
	for _, apiKey := range apiKeys {
		if err := usecase.enforceSecurity.ReadApiKey(apiKey); err != nil {
			return []models.ApiKey{}, err
		}
	}
	return apiKeys, nil
}

func (usecase *ApiKeyUseCase) getApiKey(ctx context.Context, exec repositories.Executor, apiKeyId string) (models.ApiKey, error) {
	apiKey, err := usecase.apiKeyRepository.GetApiKeyById(ctx, exec, apiKeyId)
	if err != nil {
		return models.ApiKey{}, err
	}
	if err := usecase.enforceSecurity.ReadApiKey(apiKey); err != nil {
		return models.ApiKey{}, err
	}
	return apiKey, nil
}

func (usecase *ApiKeyUseCase) CreateApiKey(ctx context.Context, input models.CreateApiKeyInput) (models.CreatedApiKey, error) {
	apiKey, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.CreatedApiKey, error) {
			if err := usecase.enforceSecurity.CreateApiKey(input.OrganizationId); err != nil {
				return models.CreatedApiKey{}, err
			}

			if input.Role != models.API_CLIENT {
				return models.CreatedApiKey{}, errors.Wrap(models.BadParameterError,
					fmt.Sprintf("role %s is not supported", input.Role))
			}

			apiKeyId := uuid.NewString()
			key := generateAPiKey()
			if err := usecase.apiKeyRepository.CreateApiKey(ctx, tx, models.CreateApiKey{
				CreateApiKeyInput: input,
				Id:                apiKeyId,
				Hash:              key, // TODO: hash the key
			}); err != nil {
				return models.CreatedApiKey{}, err
			}

			apiKey, err := usecase.getApiKey(ctx, tx, apiKeyId)
			if err != nil {
				return models.CreatedApiKey{}, err
			}
			return models.CreatedApiKey{
				ApiKey: apiKey,
				Value:  key,
			}, nil
		})
	if err != nil {
		return models.CreatedApiKey{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsApiKeyCreated, map[string]interface{}{
		"api_key_id": apiKey.Id,
	})
	return apiKey, nil
}

func generateAPiKey() string {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		panic(fmt.Errorf("randomAPiKey: %w", err))
	}
	return hex.EncodeToString(key)
}

func (usecase *ApiKeyUseCase) DeleteApiKey(ctx context.Context, apiKeyId string) error {
	exec := usecase.executorFactory.NewExecutor()
	apiKey, err := usecase.apiKeyRepository.GetApiKeyById(ctx, exec, apiKeyId)
	if err != nil {
		return err
	}

	if err := usecase.enforceSecurity.DeleteApiKey(apiKey); err != nil {
		return err
	}

	err = usecase.apiKeyRepository.SoftDeleteApiKey(ctx, exec, apiKey.Id)
	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsApiKeyDeleted, map[string]interface{}{
		"api_key_id": apiKeyId,
	})
	return nil
}
