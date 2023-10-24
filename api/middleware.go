package api

import (
	"context"
	"errors"
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

func wrapErrInUnAuthorizedError(err error) error {
	// Follow auth0 recommandation: (source https://auth0.com/blog/forbidden-unauthorized-http-status-codes)
	// When to Use 401 Unauthorized?
	// - An access token is missing.
	// - An access token is expired, revoked, malformed, or invalid for other reasons.
	if errors.Is(err, models.UnAuthorizedError) {
		return err
	}
	return errors.Join(models.UnAuthorizedError, err)
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

func HasPermission(permission models.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		credentials, ok := c.Request.Context().Value(utils.ContextKeyCredentials).(models.Credentials)
		if !ok {
			fmt.Println("hello")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if !credentials.Role.HasPermission(permission) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

type validator interface {
	Validate(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error)
}

type Authentication struct {
	validator validator
}

func (a *Authentication) Middleware(c *gin.Context) {
	key := ParseApiKeyHeader(c.Request.Header)
	jwtToken, err := ParseAuthorizationBearerHeader(c.Request.Header)
	if err != nil {
		_ = c.Error(fmt.Errorf("could not parse authorization header: %w", err))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	credentials, err := a.validator.Validate(c.Request.Context(), jwtToken, key)
	if err != nil {
		_ = c.Error(fmt.Errorf("validator.Validate error: %w", err))
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	newContext := context.WithValue(c.Request.Context(), utils.ContextKeyCredentials, credentials)
	if attr, ok := identityAttr(credentials.ActorIdentity); ok {
		logger := utils.LoggerFromContext(newContext).
			With(attr).
			With(slog.String("Role", credentials.Role.String()))
		c.Request = c.Request.WithContext(context.WithValue(newContext, utils.ContextKeyLogger, logger))
	}
	c.Next()
}

func NewAuthentication(validator validator) *Authentication {
	return &Authentication{
		validator: validator,
	}
}
