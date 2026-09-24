package usecases

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type ApiKeyRepository interface {
	GetApiKeyById(ctx context.Context, exec repositories.Executor, apiKeyId string) (models.ApiKey, error)
	ListApiKeys(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) ([]models.ApiKey, error)
	CreateApiKey(ctx context.Context, exec repositories.Executor, apiKey models.ApiKey) error
	SoftDeleteApiKey(ctx context.Context, exec repositories.Executor, apiKeyId string) error
	GetRoleBySlug(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, slug models.Role) (models.RbacRole, error)
}

type EnforceSecurityApiKey interface {
	ReadApiKey(apiKey models.ApiKey) error
	CreateApiKey(organizationId uuid.UUID) error
	DeleteApiKey(apiKey models.ApiKey) error
}

type ApiKeyUseCase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	enforceSecurity    EnforceSecurityApiKey
	apiKeyRepository   ApiKeyRepository
}

func (usecase *ApiKeyUseCase) ListApiKeys(ctx context.Context, organizationId uuid.UUID) ([]models.ApiKey, error) {
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

func (usecase *ApiKeyUseCase) CreateApiKey(ctx context.Context, input models.CreateApiKeyInput) (models.CreatedApiKey, error) {
	apiKeyId := pure_utils.NewId().String()
	key := generateAPiKey()
	hash := sha256.Sum256([]byte(key))
	if err := usecase.enforceSecurity.CreateApiKey(input.OrganizationId); err != nil {
		return models.CreatedApiKey{}, err
	}

	if len(input.RoleBindings) == 0 {
		return models.CreatedApiKey{}, errors.Wrap(models.BadParameterError, "at least one role binding is required")
	}

	bindings := append([]models.RoleBinding(nil), input.RoleBindings...)
	for idx := range bindings {
		binding := &bindings[idx]
		if binding.Role == "" {
			return models.CreatedApiKey{}, errors.Wrap(models.BadParameterError, "role binding must reference a role")
		}
		if err := binding.Conditions.Validate(); err != nil {
			return models.CreatedApiKey{}, err
		}
		if !binding.Role.IsCustom() && binding.Role != models.API_CLIENT {
			return models.CreatedApiKey{}, errors.Wrap(models.BadParameterError, "only API_CLIENT is supported as a native API key role")
		}
		if binding.Role.IsCustom() {
			if !binding.Role.IsValidCustom() {
				return models.CreatedApiKey{}, errors.Wrap(models.BadParameterError, "invalid custom role slug")
			}
			role, err := usecase.apiKeyRepository.GetRoleBySlug(
				ctx,
				usecase.executorFactory.NewExecutor(),
				input.OrganizationId,
				binding.Role,
			)
			if err != nil {
				return models.CreatedApiKey{}, err
			}
			binding.CustomRoleId = &role.Id
			binding.Permissions = role.Permissions
		} else {
			binding.CustomRoleId = nil
			binding.Permissions = binding.Role.Permissions()
		}
	}

	apiKey := models.ApiKey{
		Id:             apiKeyId,
		Description:    input.Description,
		Hash:           hash[:],
		Prefix:         key[:3],
		OrganizationId: input.OrganizationId,
		RoleBindings:   bindings,
	}

	var err error

	if usecase.transactionFactory != nil {
		err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			return usecase.apiKeyRepository.CreateApiKey(ctx, tx, apiKey)
		})
	} else {
		err = usecase.apiKeyRepository.CreateApiKey(ctx, usecase.executorFactory.NewExecutor(), apiKey)
	}

	if err != nil {
		return models.CreatedApiKey{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsApiKeyCreated, map[string]interface{}{
		"api_key_id": apiKey.Id,
	})

	return models.CreatedApiKey{
		ApiKey: apiKey,
		Key:    key,
	}, nil
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
