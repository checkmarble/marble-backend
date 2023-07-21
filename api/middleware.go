package api

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
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
	if errors.Is(err, models.UnAuthorizedError) {
		return err
	}
	return errors.Join(models.UnAuthorizedError, err)
}

// AuthCtx sets the organization ID in the context from the authorization header
func (api *API) credentialsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		key := ParseApiKeyHeader(r.Header)

		jwtToken, err := ParseAuthorizationBearerHeader(r.Header)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
			return
		}

		usecase := api.UsecasesWithCreds(r).NewMarbleTokenUseCase()
		creds, err := usecase.ValidateCredentials(jwtToken, key)
		if err != nil {
			err = wrapErrInUnAuthorizedError(err)
		}

		if presentError(w, r, err) {
			return
		}

		newContext := context.WithValue(r.Context(), utils.ContextKeyCredentials, creds)

		// Creds contain a userId or an Api key
		// create a new logger with this useful information.

		if attr, ok := identityAttr(creds.ActorIdentity); ok {
			logger := utils.LoggerFromContext(newContext).
				With(attr).
				With(slog.String("Role", creds.Role.String()))
			// store new logger in context
			newContext = context.WithValue(newContext, utils.ContextKeyLogger, logger)
		}

		next.ServeHTTP(w, r.WithContext(newContext))
	})
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

func (api *API) enforcePermissionMiddleware(permission models.Permission) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			ctx := r.Context()
			creds := utils.CredentialsFromCtx(ctx)
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
