package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type ApiKeyRepository interface {
	GetApiKeyOfOrganization(ctx context.Context, organizationId string) ([]models.ApiKey, error)
	GetApiKeyByKey(ctx context.Context, apiKey string) (models.ApiKey, error)
}
