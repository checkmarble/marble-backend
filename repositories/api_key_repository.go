package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type ApiKeyRepository interface {
	GetApiKeyOfOrganization(ctx context.Context, organizationId string) ([]models.ApiKey, error)
	GetOrganizationIDFromApiKey(ctx context.Context, apiKey string) (orgID string, err error)
}
