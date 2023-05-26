package api

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	. "marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
	"strings"

	"golang.org/x/exp/slog"
)

func ParseApiKeyHeader(header http.Header) string {
	return strings.TrimSpace(header.Get("X-API-Key"))
}

func wrapErrInUnAuthorizedError(err error) error {
	// Follow auth0 recommandation: (source https://auth0.com/blog/forbidden-unauthorized-http-status-codes)
	// When to Use 401 Unauthorized?
	// - An access token is missing.
	// - An access token is expired, revoked, malformed, or invalid for other reasons.
	if errors.Is(err, UnAuthorizedError) {
		return err
	}
	return errors.Join(UnAuthorizedError, err)
}

func (api *API) loggerMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// If there is creds and the creds contain a userId or an Api key
			// Then create a new logger with this useful information.
			if creds, ok := utils.CredentialsFromCtx(r.Context()); ok {
				if attr, ok := logAttr(creds.ActorIdentity); ok {
					logger = logger.With(attr)
				}
			}
			ctxWithToken := context.WithValue(r.Context(), utils.ContextKeyLogger, logger)
			next.ServeHTTP(w, r.WithContext(ctxWithToken))
		})
	}
}

func logAttr(identity models.Identity) (attr slog.Attr, ok bool) {
	if identity.ApiKeyName != "" {
		return slog.String("ApiKeyName", identity.ApiKeyName), true
	}
	if identity.UserId != "" {
		return slog.String("UserId", identity.UserId), true
	}
	return slog.Attr{}, false
}

// AuthCtx sets the organization ID in the context from the authorization header
func (api *API) credentialsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		apiKey := ParseApiKeyHeader(r.Header)

		jwtToken, err := ParseAuthorizationBearerHeader(r.Header)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
			return
		}

		usecase := api.usecases.NewMarbleTokenUseCase()
		ctx := r.Context()
		creds, err := usecase.ValidateCredentials(ctx, jwtToken, apiKey)
		if err != nil {
			err = wrapErrInUnAuthorizedError(err)
		}

		if presentError(w, r, err) {
			return
		}

		ctxWithToken := context.WithValue(ctx, utils.ContextKeyCredentials, creds)
		next.ServeHTTP(w, r.WithContext(ctxWithToken))
	})
}

func (api *API) enforcePermissionMiddleware(permission Permission) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			ctx := r.Context()
			creds := utils.MustCredentialsFromCtx(ctx)
			allowed := creds.Role.HasPermission(permission)

			if allowed {
				next.ServeHTTP(w, r)
			} else {
				errorMessage := fmt.Sprintf("Missing permission %s", permission.String())
				api.logger.WarnCtx(ctx, errorMessage)
				http.Error(w, errorMessage, http.StatusForbidden)
			}
		})
	}
}
