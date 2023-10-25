package apikeys

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type mockApiKeysRepository struct {
	mock.Mock
}

func (m *mockApiKeysRepository) GetApiKeysOfOrganization(ctx context.Context, organizationID string) ([]models.ApiKey, error) {
	args := m.Called(ctx, organizationID)
	return args.Get(0).([]models.ApiKey), args.Error(1)
}

func TestUseCase_GetApiKeysOfOrganization(t *testing.T) {
	organizationID := uuid.NewString()
	keys := []models.ApiKey{
		{
			ApiKeyId:       models.ApiKeyId(uuid.NewString()),
			OrganizationId: uuid.NewString(),
			Key:            uuid.NewString(),
			Role:           models.BUILDER,
		},
		{
			ApiKeyId:       models.ApiKeyId(uuid.NewString()),
			OrganizationId: uuid.NewString(),
			Key:            uuid.NewString(),
			Role:           models.PUBLISHER,
		},
	}

	t.Run("nominal", func(t *testing.T) {
		repository := new(mockApiKeysRepository)
		repository.On("GetApiKeysOfOrganization", mock.Anything, organizationID).
			Return(keys, nil)

		useCase := UseCase{
			repository: repository,
		}

		apiKeys, err := useCase.GetApiKeysOfOrganization(context.Background(), organizationID)
		assert.Equal(t, keys, apiKeys)
		assert.NoError(t, err)
		repository.AssertExpectations(t)
	})

	t.Run("GetApiKeysOfOrganization error", func(t *testing.T) {
		repository := new(mockApiKeysRepository)
		repository.On("GetApiKeysOfOrganization", mock.Anything, organizationID).
			Return([]models.ApiKey{}, assert.AnError)

		useCase := UseCase{
			repository: repository,
		}

		_, err := useCase.GetApiKeysOfOrganization(context.Background(), organizationID)
		assert.Error(t, err)
		repository.AssertExpectations(t)
	})
}
