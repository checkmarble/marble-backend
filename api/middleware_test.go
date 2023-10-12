package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("nominal", func(t *testing.T) {
		credentials := models.Credentials{
			OrganizationId: "organization",
			Role:           models.ADMIN,
			ActorIdentity: models.Identity{
				UserId: "user_id",
				Email:  "user@email.com",
			},
		}

		okHandlerValidateCredentials := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds := r.Context().Value(utils.ContextKeyCredentials).(models.Credentials)
			assert.Equal(t, credentials, creds)
			w.WriteHeader(http.StatusOK)
		})

		mValidator := new(mockValidator)
		mValidator.On("Validate", mock.Anything, "token", "").
			Return(credentials, nil)

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", nil)
		req.Header.Add("Authorization", "Bearer token")

		res := httptest.NewRecorder()

		m := Authentication{
			validator: mValidator,
		}
		handler := m.Middleware(okHandlerValidateCredentials)
		handler.ServeHTTP(res, req)
		assert.Equal(t, http.StatusOK, res.Code)
		mValidator.AssertExpectations(t)
	})

	t.Run("Validate error", func(t *testing.T) {
		mValidator := new(mockValidator)
		mValidator.On("Validate", mock.Anything, "token", "").
			Return(models.Credentials{}, assert.AnError)

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", nil)
		req.Header.Add("Authorization", "Bearer token")

		res := httptest.NewRecorder()

		m := Authentication{
			validator: mValidator,
		}
		handler := m.Middleware(okHandler)
		handler.ServeHTTP(res, req)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
		mValidator.AssertExpectations(t)
	})

	t.Run("bad token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", nil)
		req.Header.Add("Authorization", "bad")

		res := httptest.NewRecorder()

		m := Authentication{}
		handler := m.Middleware(okHandler)
		handler.ServeHTTP(res, req)
		assert.Equal(t, http.StatusBadRequest, res.Code)
	})
}
