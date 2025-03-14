package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

type tokenGenerator interface {
	GenerateToken(ctx context.Context, key string, firebaseToken string) (string, time.Time, error)
}

type TokenHandler struct {
	generator tokenGenerator
}

type accessToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (t *TokenHandler) GenerateToken(c *gin.Context) {
	ctx := c.Request.Context()
	key := utils.ParseApiKeyHeader(c.Request.Header)
	bearerToken, err := ParseAuthorizationBearerHeader(c.Request.Header)
	if err != nil {
		_ = c.Error(fmt.Errorf("could not parse authorization header: %w", err))
		c.Status(http.StatusBadRequest)
		return
	}

	marbleToken, expirationTime, err := t.generator.GenerateToken(ctx, key, bearerToken)
	if err != nil {
		_ = c.Error(fmt.Errorf("generator.GenerateToken error: %w", err))

		if errors.Is(err, models.ErrUnknownUser) {
			c.JSON(http.StatusUnauthorized, dto.APIErrorResponse{
				Message:   "Unknown user: ErrUnknownUser",
				ErrorCode: dto.UnknownUser,
			})
			return
		}

		c.JSON(http.StatusUnauthorized, dto.APIErrorResponse{
			Message: "Authentication error",
		})
		return
	}

	c.JSON(http.StatusOK, accessToken{
		AccessToken: marbleToken,
		TokenType:   "Bearer",
		ExpiresAt:   expirationTime,
	})
}

func NewTokenHandler(generator tokenGenerator) TokenHandler {
	return TokenHandler{
		generator: generator,
	}
}
