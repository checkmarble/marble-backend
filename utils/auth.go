package utils

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/models"
)

type AuthType int

const (
	FederatedBearerToken AuthType = iota
	PublicApiKey
	BearerToken
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
	Validator validator
}

func (a *Authentication) AuthedBy(methods ...AuthType) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		key := ""
		jwtToken := ""

		if slices.Contains(methods, PublicApiKey) {
			key = ParseApiKeyHeader(c.Request.Header)
		}

		if key == "" && slices.Contains(methods, BearerToken) {
			token, err := ParseAuthorizationBearerHeader(c.Request.Header)
			if err != nil {
				_ = c.Error(fmt.Errorf("could not parse authorization header: %w", err))
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			key = token
		}

		if slices.Contains(methods, FederatedBearerToken) {
			token, err := ParseAuthorizationBearerHeader(c.Request.Header)
			if err != nil {
				_ = c.Error(fmt.Errorf("could not parse authorization header: %w", err))
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			jwtToken = token
		}

		credentials, err := a.Validator.Validate(ctx, jwtToken, key)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				_ = c.Error(fmt.Errorf("validator.Validate error: %w", err))
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			LogAndReportSentryError(ctx, err)
			LoggerFromContext(ctx).ErrorContext(ctx,
				"errors while validating token", "error", err)

			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		newContext := context.WithValue(ctx, ContextKeyCredentials, credentials)
		if attr, ok := identityAttr(credentials.ActorIdentity); ok {
			logger := LoggerFromContext(newContext).
				With(attr).
				With(slog.String("Role", credentials.Role.String()))
			c.Request = c.Request.WithContext(context.WithValue(newContext, ContextKeyLogger, logger))
		}
		c.Next()
	}
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

	credentials, err := a.Validator.Validate(ctx, jwtToken, key)
	if err != nil {
		_ = c.Error(fmt.Errorf("validator.Validate error: %w", err))
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	newContext := context.WithValue(ctx, ContextKeyCredentials, credentials)
	if attr, ok := identityAttr(credentials.ActorIdentity); ok {
		logger := LoggerFromContext(newContext).
			With(attr).
			With(slog.String("Role", credentials.Role.String()))
		c.Request = c.Request.WithContext(context.WithValue(newContext, ContextKeyLogger, logger))
	}
	c.Next()
}

func NewAuthentication(validator validator) Authentication {
	return Authentication{
		Validator: validator,
	}
}

func ParseAuthorizationBearerHeader(header http.Header) (string, error) {
	authorization := header.Get("Authorization")
	if authorization == "" {
		return "", nil
	}

	authHeader := strings.Split(header.Get("Authorization"), "Bearer ")
	if len(authHeader) != 2 {
		return "", fmt.Errorf("malformed token: %w", models.UnAuthorizedError)
	}
	return authHeader[1], nil
}
