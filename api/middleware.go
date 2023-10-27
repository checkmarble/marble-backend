package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

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

type validator interface {
	Validate(ctx context.Context, marbleToken, apiKey string) (models.Credentials, error)
}

type Authentication struct {
	validator validator
}

func (a *Authentication) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := ParseApiKeyHeader(r.Header)
		jwtToken, err := ParseAuthorizationBearerHeader(r.Header)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
			return
		}

		credentials, err := a.validator.Validate(r.Context(), jwtToken, key)
		if err != nil {
			err = wrapErrInUnAuthorizedError(err)
		}
		if presentError(w, r, err) {
			return
		}

		newContext := context.WithValue(r.Context(), utils.ContextKeyCredentials, credentials)
		if attr, ok := identityAttr(credentials.ActorIdentity); ok {
			logger := utils.LoggerFromContext(newContext).
				With(attr).
				With(slog.String("Role", credentials.Role.String()))
			newContext = context.WithValue(newContext, utils.ContextKeyLogger, logger)
		}
		next.ServeHTTP(w, r.WithContext(newContext))
	})
}

func NewAuthentication(validator validator) *Authentication {
	return &Authentication{
		validator: validator,
	}
}
