package api

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type tokenGenerator interface {
	GenerateToken(ctx context.Context, key string, firebaseToken string) (string, time.Time, error)
}

type TokenHandler struct {
	generator tokenGenerator
}

type token struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (t *TokenHandler) GenerateToken(w http.ResponseWriter, request *http.Request) {
	key := ParseApiKeyHeader(request.Header)
	bearerToken, err := ParseAuthorizationBearerHeader(request.Header)
	if err != nil {
		http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
		return
	}

	marbleToken, expirationTime, err := t.generator.GenerateToken(request.Context(), key, bearerToken)
	if err != nil {
		err = wrapErrInUnAuthorizedError(err)
	}
	if presentError(w, request, err) {
		return
	}

	PresentModel(w, token{
		AccessToken: marbleToken,
		TokenType:   "Bearer",
		ExpiresAt:   expirationTime,
	})
}

func NewTokenHandler(generator tokenGenerator) *TokenHandler {
	return &TokenHandler{
		generator: generator,
	}
}
