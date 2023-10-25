package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type mockApiKeysUseCase struct {
	mock.Mock
}

func (m *mockApiKeysUseCase) GetApiKeysOfOrganization(ctx context.Context, organizationID string) ([]models.ApiKey, error) {
	args := m.Called(ctx, organizationID)
	return args.Get(0).([]models.ApiKey), args.Error(1)
}

func TestApiKeysHandler_GetApiKeys(t *testing.T) {
	organizationID := uuid.NewString()
	credentials := models.Credentials{
		OrganizationId: organizationID,
		Role:           models.ADMIN,
	}
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, credentials)

	keys := []models.ApiKey{
		{
			ApiKeyId:       models.ApiKeyId(uuid.NewString()),
			OrganizationId: uuid.NewString(),
			Key:            uuid.NewString(),
			Role:           models.ADMIN,
		},
		{
			ApiKeyId:       models.ApiKeyId(uuid.NewString()),
			OrganizationId: uuid.NewString(),
			Key:            uuid.NewString(),
			Role:           models.VIEWER,
		},
	}

	t.Run("nominal", func(t *testing.T) {
		useCase := new(mockApiKeysUseCase)
		useCase.On("GetApiKeysOfOrganization", mock.Anything, organizationID).
			Return(keys, nil)

		handler := ApiKeysHandler{
			useCase: useCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/keys", handler.GetApiKeys)

		req := httptest.NewRequest(http.MethodPost, "https://www.checkmarble.com/keys", nil).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)

		expected, _ := json.Marshal(map[string]interface{}{
			"api_keys": utils.Map(keys, dto.AdaptApiKeyDto),
		})
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, string(expected), r.Body.String())
		useCase.AssertExpectations(t)
	})

	t.Run("GetApiKeysOfOrganization error", func(t *testing.T) {
		useCase := new(mockApiKeysUseCase)
		useCase.On("GetApiKeysOfOrganization", mock.Anything, organizationID).
			Return([]models.ApiKey{}, assert.AnError)

		handler := ApiKeysHandler{
			useCase: useCase,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/keys", handler.GetApiKeys)

		req := httptest.NewRequest(http.MethodGet, "https://www.checkmarble.com/keys", nil).
			WithContext(ctx)

		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)

		assert.Equal(t, http.StatusInternalServerError, r.Code)
		useCase.AssertExpectations(t)
	})
}
