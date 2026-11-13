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
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ApiKeyRepository interface {
	GetApiKeyById(ctx context.Context, exec repositories.Executor, apiKeyId string) (models.ApiKey, error)
	ListApiKeys(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) ([]models.ApiKey, error)
	CreateApiKey(ctx context.Context, exec repositories.Executor, apiKey models.ApiKey) error
	SoftDeleteApiKey(ctx context.Context, exec repositories.Executor, apiKeyId string) error
}

type EnforceSecurityApiKey interface {
	ReadApiKey(apiKey models.ApiKey) error
	CreateApiKey(organizationId uuid.UUID, role models.Role) error
	DeleteApiKey(apiKey models.ApiKey) error
	CheckApiKeyTenant(tenantID uuid.UUID) error
}

type apiKeyGrantRepository interface {
	CreateApiKeyGrant(context.Context, repositories.Transaction, string, models.Grant) error
	RevokeApiKeyGrants(context.Context, repositories.Transaction, string) error
}

type apiKeyOrganizationRepository interface {
	LockOrganizationTenant(context.Context, repositories.Transaction, uuid.UUID) (uuid.UUID, error)
}

type ApiKeyUseCase struct {
	executorFactory        executor_factory.ExecutorFactory
	transactionFactory     executor_factory.TransactionFactory
	enforceSecurity        EnforceSecurityApiKey
	apiKeyRepository       ApiKeyRepository
	grantRepository        apiKeyGrantRepository
	organizationRepository apiKeyOrganizationRepository
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
	apiKey := models.ApiKey{
		Id:             apiKeyId,
		Description:    input.Description,
		Hash:           hash[:],
		Prefix:         key[:3],
		OrganizationId: input.OrganizationId,
		Role:           input.Role,
	}

	if err := usecase.enforceSecurity.CreateApiKey(input.OrganizationId, input.Role); err != nil {
		return models.CreatedApiKey{}, err
	}
	if input.Role != models.API_CLIENT && input.Role != models.OIDC_CONFIG_EDITOR {
		return models.CreatedApiKey{}, errors.Wrap(
			models.BadParameterError,
			fmt.Sprintf("role %s is not supported", input.Role),
		)
	}

	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		grant := models.Grant{Role: input.Role, OrganizationId: input.OrganizationId}
		if input.Role == models.OIDC_CONFIG_EDITOR {
			tenantID, err := usecase.organizationRepository.LockOrganizationTenant(ctx, tx, input.OrganizationId)
			if err != nil {
				return err
			}
			if err := usecase.enforceSecurity.CheckApiKeyTenant(tenantID); err != nil {
				return err
			}
			grant.OrganizationId, grant.TenantId = uuid.Nil, tenantID
		}
		if err := usecase.apiKeyRepository.CreateApiKey(ctx, tx, apiKey); err != nil {
			return err
		}
		return usecase.grantRepository.CreateApiKeyGrant(ctx, tx, apiKey.Id, grant)
	})
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
		panic(errors.Errorf("generateAPiKey: %w", err))
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

	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.apiKeyRepository.SoftDeleteApiKey(ctx, tx, apiKey.Id); err != nil {
			return err
		}
		return usecase.grantRepository.RevokeApiKeyGrants(ctx, tx, apiKey.Id)
	})
	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsApiKeyDeleted, map[string]interface{}{
		"api_key_id": apiKeyId,
	})
	return nil
}
