package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

type TokenHandler struct {
	handler auth.TokenHandler
}

type accessToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (t *TokenHandler) GenerateToken(c *gin.Context) {
	ctx := c.Request.Context()
	token, err := t.handler.GetToken(ctx, c.Request)

	if err != nil {
		utils.LoggerFromContext(ctx).ErrorContext(ctx, "could not verify firebase token", "error", err)

		_ = c.Error(fmt.Errorf("generator.GenerateToken error: %w", err))

		if errors.Is(err, models.ErrUnknownUser) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.APIErrorResponse{
				Message:   "Unknown user: ErrUnknownUser",
				ErrorCode: dto.UnknownUser,
			})
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, dto.APIErrorResponse{
			Message: "Authentication error",
		})
		return
	}

	newContext := context.WithValue(ctx, utils.ContextKeyCredentials, token.Credentials)

	c.Request = c.Request.WithContext(newContext)

	c.JSON(http.StatusOK, accessToken{
		AccessToken: token.Value,
		TokenType:   "Bearer",
		ExpiresAt:   token.Expiration,
	})
}

func NewTokenHandler(handler auth.TokenHandler) TokenHandler {
	return TokenHandler{
		handler: handler,
	}
}
