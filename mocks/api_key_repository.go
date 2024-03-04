package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ApiKeyRepository struct {
	mock.Mock
}

func (r *ApiKeyRepository) GetApiKeyById(ctx context.Context, exec repositories.Executor, apiKeyId string) (models.ApiKey, error) {
	args := r.Called(exec, apiKeyId)
	return args.Get(0).(models.ApiKey), args.Error(1)
}

func (r *ApiKeyRepository) ListApiKeys(ctx context.Context, exec repositories.Executor, organizationId string) ([]models.ApiKey, error) {
	args := r.Called(exec, organizationId)
	return args.Get(0).([]models.ApiKey), args.Error(1)
}

func (r *ApiKeyRepository) CreateApiKey(ctx context.Context, exec repositories.Executor, apiKey models.ApiKey) error {
	args := r.Called(exec, apiKey)
	return args.Error(0)
}

func (r *ApiKeyRepository) SoftDeleteApiKey(ctx context.Context, exec repositories.Executor, apiKeyId string) error {
	args := r.Called(exec, apiKeyId)
	return args.Error(0)
}
