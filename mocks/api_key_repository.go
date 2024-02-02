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

func (r *ApiKeyRepository) GetApiKeyById(ctx context.Context, tx repositories.Transaction, apiKeyId string) (models.ApiKey, error) {
	args := r.Called(tx, apiKeyId)
	return args.Get(0).(models.ApiKey), args.Error(1)
}

func (r *ApiKeyRepository) ListApiKeys(ctx context.Context, tx repositories.Transaction, organizationId string) ([]models.ApiKey, error) {
	args := r.Called(tx, organizationId)
	return args.Get(0).([]models.ApiKey), args.Error(1)
}

func (r *ApiKeyRepository) CreateApiKey(ctx context.Context, tx repositories.Transaction, apiKey models.CreateApiKey) error {
	args := r.Called(tx, apiKey)
	return args.Error(0)
}

func (r *ApiKeyRepository) SoftDeleteApiKey(ctx context.Context, tx repositories.Transaction, apiKeyId string) error {
	args := r.Called(tx, apiKeyId)
	return args.Error(0)
}
