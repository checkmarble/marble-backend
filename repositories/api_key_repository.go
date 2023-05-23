package repositories

import "context"

type ApiKeyRepository interface {
	GetOrganizationIDFromApiKey(ctx context.Context, apiKey string) (orgID string, err error)
}
