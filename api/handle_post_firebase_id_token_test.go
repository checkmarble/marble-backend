package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockGenerator struct {
	mock.Mock
}

func (m *mockGenerator) GenerateToken(ctx context.Context, key string, firebaseToken string) (string, time.Time, error) {
	args := m.Called(ctx, key, firebaseToken)
	return args.String(0), args.Get(1).(time.Time), args.Error(2)
}

func TestToken_GenerateToken(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		tok := token{
			AccessToken: "marbleToken",
			TokenType:   "Bearer",
			ExpiresAt:   time.Now(),
		}

		mGenerator := new(mockGenerator)
		mGenerator.On("GenerateToken", mock.Anything, "", "token").
			Return(tok.AccessToken, tok.ExpiresAt, nil)

		tokenHandler := TokenHandler{
			generator: mGenerator,
		}

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", nil)
		req.Header.Add("Authorization", "Bearer token")

		res := httptest.NewRecorder()

		handler := http.HandlerFunc(tokenHandler.GenerateToken)
		handler.ServeHTTP(res, req)

		data, _ := json.Marshal(&tok)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.JSONEq(t, string(data), res.Body.String())
		mGenerator.AssertExpectations(t)
	})

	t.Run("GenerateToken error", func(t *testing.T) {
		mGenerator := new(mockGenerator)
		mGenerator.On("GenerateToken", mock.Anything, "", "token").
			Return("", time.Time{}, assert.AnError)

		tokenHandler := TokenHandler{
			generator: mGenerator,
		}

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", nil)
		req.Header.Add("Authorization", "Bearer token")

		res := httptest.NewRecorder()

		handler := http.HandlerFunc(tokenHandler.GenerateToken)
		handler.ServeHTTP(res, req)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
		mGenerator.AssertExpectations(t)
	})

	t.Run("bad token", func(t *testing.T) {
		tokenHandler := TokenHandler{}

		req := httptest.NewRequest(http.MethodGet, "http://www.checkmarble.com", nil)
		req.Header.Add("Authorization", "bad")

		res := httptest.NewRecorder()

		handler := http.HandlerFunc(tokenHandler.GenerateToken)
		handler.ServeHTTP(res, req)
		assert.Equal(t, http.StatusBadRequest, res.Code)
	})
}
