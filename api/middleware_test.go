package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type mockValidator struct {
	mock.Mock
}

func (m *mockValidator) Validate(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error) {
	args := m.Called(ctx, marbleToken, apiKey)
	return args.Get(0).(models.Credentials), args.Error(1)
}

func TestAuthentication_Middleware(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		credentials := models.Credentials{
			OrganizationId: "organization",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: "user_id",
				Email:  "user@email.com",
			},
		}

		okHandlerValidateCredentials := func(c *gin.Context) {
			creds := c.Request.Context().Value(utils.ContextKeyCredentials).(models.Credentials)
			assert.Equal(t, credentials, creds)
			c.Status(http.StatusOK)
		}

		mValidator := new(mockValidator)
		mValidator.On("Validate", mock.Anything, "token", "").
			Return(credentials, nil)

		res := httptest.NewRecorder()

		m := Authentication{
			validator: mValidator,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/test", m.Middleware, okHandlerValidateCredentials)

		req := httptest.NewRequest(http.MethodGet, "https://checkmarble.com/test", nil)
		req.Header.Add("Authorization", "Bearer token")

		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)

		assert.Equal(t, http.StatusOK, res.Code)
		mValidator.AssertExpectations(t)
	})

	t.Run("Validate error", func(t *testing.T) {
		mValidator := new(mockValidator)
		mValidator.On("Validate", mock.Anything, "token", "").
			Return(models.Credentials{}, assert.AnError)

		m := Authentication{
			validator: mValidator,
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/test", m.Middleware)

		req := httptest.NewRequest(http.MethodGet, "https://checkmarble.com/test", nil)
		req.Header.Add("Authorization", "Bearer token")

		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)

		assert.Equal(t, http.StatusUnauthorized, r.Code)
		mValidator.AssertExpectations(t)
	})

	t.Run("bad token", func(t *testing.T) {
		m := Authentication{}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/test", m.Middleware)

		req := httptest.NewRequest(http.MethodGet, "https://checkmarble.com/test", nil)
		req.Header.Add("Authorization", "bad")

		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)

		assert.Equal(t, http.StatusBadRequest, r.Code)
	})
}
