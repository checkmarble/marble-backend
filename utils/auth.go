package utils

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set/v2"

	"github.com/gin-gonic/gin"
)

type AuthType int

const (
	FederatedBearerToken AuthType = iota
	PublicApiKey
	ApiKeyAsBearerToken
	ScreeningIndexerToken
)

const screeningIndexerTokenPrefix = "Token "

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

type tokenAndKeyValidator interface {
	ValidateTokenOrKey(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error)
}

type roleBindingFetcher interface {
	FetchRoleBindings(ctx context.Context, orgId uuid.UUID, userId, apiKeyId string) ([]models.RoleBinding, error)
}

type Authentication struct {
	validator             tokenAndKeyValidator
	roleBindingFetcher    roleBindingFetcher
	screeningIndexerToken string
}

func (a *Authentication) AuthedBy(methods ...AuthType) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If the middleware is executing, but no route was matched, we
		// short-circuit with a 404.
		if c.FullPath() == "" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		ctx := c.Request.Context()

		key := ""
		jwtToken := ""

		if slices.Contains(methods, ScreeningIndexerToken) {
			token, err := ParseAuthorizationTokenHeader(c.Request.Header)
			if err != nil {
				_ = c.Error(fmt.Errorf("could not parse authorization header: %w", err))
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			if token != "" && token == a.screeningIndexerToken {
				c.Next()
				return
			}
		}

		if slices.Contains(methods, PublicApiKey) {
			key = ParseApiKeyHeader(c.Request.Header)
		}

		if key == "" && slices.Contains(methods, ApiKeyAsBearerToken) {
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

		if key == "" && jwtToken == "" {
			_ = c.Error(fmt.Errorf("missing authentication method"))
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		credentials, err := a.validator.ValidateTokenOrKey(ctx, jwtToken, key)
		if err != nil {
			if errors.Is(err, models.NotFoundError) ||
				errors.Is(err, models.UnAuthorizedError) {
				_ = c.Error(fmt.Errorf("validator.Validate error: %w", err))
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			LogAndReportSentryError(ctx, err)
			LoggerFromContext(ctx).ErrorContext(ctx, "errors while validating token", "error", err)

			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		bindings := credentials.RoleBindings
		if a.roleBindingFetcher != nil {
			bindings, err = a.roleBindingFetcher.FetchRoleBindings(
				ctx,
				credentials.OrganizationId,
				string(credentials.ActorIdentity.UserId),
				credentials.ActorIdentity.ApiKeyId,
			)
			if err != nil {
				LoggerFromContext(ctx).ErrorContext(ctx, "could not retrieve role bindings for principal", "error", err.Error())
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}
		credentials.RoleBindings = bindings
		permissions := set.New[models.Permission](10)
		activeRoles := set.New[models.Role](len(bindings))
		now := time.Now()
		for _, binding := range bindings {
			if binding.IsActive(now) {
				permissions.InsertSlice(binding.Permissions)
				if role := binding.RoleName(); role != "" {
					activeRoles.Insert(role)
				}
			}
		}
		credentials.Permissions = permissions.Slice()

		newContext := context.WithValue(ctx, ContextKeyCredentials, credentials)
		if attr, ok := identityAttr(credentials.ActorIdentity); ok {
			logger := LoggerFromContext(newContext).
				With(attr).
				With(slog.Any("Roles", activeRoles.Slice()))
			c.Request = c.Request.WithContext(context.WithValue(newContext, ContextKeyLogger, logger))
		}
		c.Next()
	}
}

func NewAuthentication(validator tokenAndKeyValidator, roleBindingFetcher roleBindingFetcher, screeningIndexerToken string) Authentication {
	return Authentication{
		validator:             validator,
		roleBindingFetcher:    roleBindingFetcher,
		screeningIndexerToken: screeningIndexerToken,
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

func ParseAuthorizationTokenHeader(header http.Header) (string, error) {
	authorization := header.Get("Authorization")
	if authorization == "" {
		return "", nil
	}

	token, found := strings.CutPrefix(authorization, screeningIndexerTokenPrefix)
	if !found {
		return "", fmt.Errorf("malformed token: %w", models.UnAuthorizedError)
	}
	return token, nil
}
