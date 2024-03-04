package usecases

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	CreateApiKey(ctx context.Context, exec repositories.Executor, apiKey models.ApiKey) error
	SoftDeleteApiKey(ctx context.Context, exec repositories.Executor, apiKeyId string) error
}

type EnforceSecurityApiKey interface {
	ReadApiKey(apiKey models.ApiKey) error
	CreateApiKey(organizationId string) error
	DeleteApiKey(apiKey models.ApiKey) error
}

type ApiKeyUseCase struct {
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

func (usecase *ApiKeyUseCase) CreateApiKey(ctx context.Context, input models.CreateApiKeyInput) (models.ApiKey, error) {
	apiKeyId := uuid.NewString()
	key := generateAPiKey()
	hash := sha256.Sum256([]byte(key))
	apiKey := models.ApiKey{
		Id:             apiKeyId,
		Description:    input.Description,
		Hash:           hash[:],
		Key:            key,
		OrganizationId: input.OrganizationId,
		Role:           input.Role,
	}

	if err := usecase.enforceSecurity.CreateApiKey(input.OrganizationId); err != nil {
		return models.ApiKey{}, err
	}

	if input.Role != models.API_CLIENT {
		return models.ApiKey{}, errors.Wrap(
			models.BadParameterError,
			fmt.Sprintf("role %s is not supported", input.Role),
		)
	}

	err := usecase.apiKeyRepository.CreateApiKey(
		ctx,
		usecase.executorFactory.NewExecutor(),
		apiKey,
	)
	if err != nil {
		return models.ApiKey{}, err
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
		panic(fmt.Errorf("generateAPiKey: %w", err))
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
