package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockGenerator struct {
	mock.Mock
}

func (m *mockGenerator) GenerateToken(ctx context.Context, creds auth.Credentials, intoCredentials models.IntoCredentials, claims models.IdentityClaims) (auth.Token, error) {
	args := m.Called(ctx, creds, claims)
	return args.Get(0).(auth.Token), args.Error(1)
}

func TestToken_GenerateToken(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		tok := accessToken{
			AccessToken: "marbleToken",
			TokenType:   "Bearer",
			ExpiresAt:   time.Now(),
		}

		mGenerator := new(mockGenerator)
		mGenerator.On("GenerateToken", mock.Anything, auth.Credentials{Value: "token"}, mock.Anything).
			Return(auth.Token{Value: tok.AccessToken, Expiration: tok.ExpiresAt}, nil)

		tokenHandler := NewTokenHandler(auth.NewTokenHandler(
			auth.DefaultExtractor(),
			mocks.NewStaticTokenVerifier("token", nil),
			mGenerator,
		))

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/token", tokenHandler.GenerateToken)

		req := httptest.NewRequest(http.MethodPost, "https://www.checkmarble.com/token", nil)
		req.Header.Add("Authorization", "Bearer token")

		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)

		data, _ := json.Marshal(&tok)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, string(data), r.Body.String())
		mGenerator.AssertExpectations(t)
	})

	t.Run("GenerateToken error", func(t *testing.T) {
		mGenerator := new(mockGenerator)
		mGenerator.On("GenerateToken", mock.Anything, auth.Credentials{Value: "token"}, mock.Anything).
			Return(auth.Token{}, assert.AnError)

		tokenHandler := NewTokenHandler(auth.NewTokenHandler(
			auth.DefaultExtractor(),
			mocks.NewStaticTokenVerifier("token", nil),
			mGenerator,
		))

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/token", tokenHandler.GenerateToken)

		req := httptest.NewRequest(http.MethodPost, "https://www.checkmarble.com/token", nil)
		req.Header.Add("Authorization", "Bearer token")

		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)

		assert.Equal(t, http.StatusUnauthorized, r.Code)
		mGenerator.AssertExpectations(t)
	})

	t.Run("bad token", func(t *testing.T) {
		tokenHandler := NewTokenHandler(auth.NewTokenHandler(
			auth.DefaultExtractor(),
			mocks.NewStaticTokenVerifier("token", nil),
			nil,
		))

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/token", tokenHandler.GenerateToken)

		req := httptest.NewRequest(http.MethodPost, "http://www.checkmarble.com/token", nil)
		req.Header.Add("Authorization", "bad")

		r := httptest.NewRecorder()
		router.ServeHTTP(r, req)

		assert.Equal(t, http.StatusUnauthorized, r.Code)
	})
}
