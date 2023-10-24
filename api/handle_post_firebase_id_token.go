package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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

func (t *TokenHandler) GenerateToken(c *gin.Context) {
	key := ParseApiKeyHeader(c.Request.Header)
	bearerToken, err := ParseAuthorizationBearerHeader(c.Request.Header)
	if err != nil {
		_ = c.Error(fmt.Errorf("could not parse authorization header: %w", err))
		c.Status(http.StatusBadRequest)
		return
	}

	marbleToken, expirationTime, err := t.generator.GenerateToken(c.Request.Context(), key, bearerToken)
	if err != nil {
		_ = c.Error(fmt.Errorf("generator.GenerateToken error: %w", err))
		c.Status(http.StatusUnauthorized)
		return
	}

	c.JSON(http.StatusOK, token{
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
