package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func ParseApiKeyHeader(header http.Header) string {
	return strings.TrimSpace(header.Get("X-API-Key"))
}

func identityAttr(identity models.Identity) (attr slog.Attr, ok bool) {
	if identity.ApiKeyName != "" {
		return slog.String("ApiKeyName", identity.ApiKeyName), true
	}
	if identity.Email != "" {
		return slog.String("Email", identity.Email), true
	}
	return slog.Attr{}, false
}

type validator interface {
	Validate(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error)
}

type Authentication struct {
	validator validator
}

func (a *Authentication) Middleware(c *gin.Context) {
	ctx := c.Request.Context()
	key := ParseApiKeyHeader(c.Request.Header)
	jwtToken, err := ParseAuthorizationBearerHeader(c.Request.Header)
	if err != nil {
		_ = c.Error(fmt.Errorf("could not parse authorization header: %w", err))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	credentials, err := a.validator.Validate(ctx, jwtToken, key)
	if err != nil {
		_ = c.Error(fmt.Errorf("validator.Validate error: %w", err))
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	newContext := context.WithValue(ctx, utils.ContextKeyCredentials, credentials)
	if attr, ok := identityAttr(credentials.ActorIdentity); ok {
		logger := utils.LoggerFromContext(newContext).
			With(attr).
			With(slog.String("Role", credentials.Role.String()))
		c.Request = c.Request.WithContext(context.WithValue(newContext, utils.ContextKeyLogger, logger))
	}
	c.Next()
}

func NewAuthentication(validator validator) Authentication {
	return Authentication{
		validator: validator,
	}
}
