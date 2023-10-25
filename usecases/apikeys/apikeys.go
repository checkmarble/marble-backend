package apikeys

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
)

type apiKeysRepository interface {
	GetApiKeysOfOrganization(ctx context.Context, organizationID string) ([]models.ApiKey, error)
}

type UseCase struct {
	repository apiKeysRepository
}

func (u *UseCase) GetApiKeysOfOrganization(ctx context.Context, organizationID string) ([]models.ApiKey, error) {
	keys, err := u.repository.GetApiKeysOfOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("repository.GetApiKeysOfOrganization error: %w", err)
	}
	return keys, nil
}

func New(repository apiKeysRepository) *UseCase {
	return &UseCase{
		repository: repository,
	}
}
