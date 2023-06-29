package middlewares

import (
	"context"
	"errors"
	"fmt"
	. "marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
	"strings"

	"golang.org/x/exp/slog"
)

func ParseApiKeyHeader(header http.Header) string {
	return strings.TrimSpace(header.Get("X-API-Key"))
}

func WrapErrInUnAuthorizedError(err error) error {
	// Follow auth0 recommandation: (source https://auth0.com/blog/forbidden-unauthorized-http-status-codes)
	// When to Use 401 Unauthorized?
	// - An access token is missing.
	// - An access token is expired, revoked, malformed, or invalid for other reasons.
	if errors.Is(err, UnAuthorizedError) {
		return err
	}
	return errors.Join(UnAuthorizedError, err)
}

func identityAttr(identity Identity) (attr slog.Attr, ok bool) {
	if identity.ApiKeyName != "" {
		return slog.String("ApiKeyName", identity.ApiKeyName), true
	}
	if identity.Email != "" {
		return slog.String("Email", identity.Email), true
	}
	return slog.Attr{}, false
}

// AuthCtx sets the organization ID in the context from the authorization header
func (mid *Middlewares) CredentialsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		key := ParseApiKeyHeader(r.Header)

		jwtToken, err := utils.ParseAuthorizationBearerHeader(r.Header)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
			return
		}

		usecase := mid.usecases.NewMarbleTokenUseCase()
		ctx := r.Context()
		creds, err := usecase.ValidateCredentials(ctx, jwtToken, key)
		if err != nil {
			err = WrapErrInUnAuthorizedError(err)
		}

		if utils.PresentError(w, r, err) {
			return
		}

		newContext := context.WithValue(ctx, utils.ContextKeyCredentials, creds)

		// Creds contain a userId or an Api key
		// create a new logger with this useful information.
		if attr, ok := identityAttr(creds.ActorIdentity); ok {
			logger := utils.LoggerFromContext(newContext).With(attr)
			// store new logger in context
			newContext = context.WithValue(newContext, utils.ContextKeyLogger, logger)
		}

		next.ServeHTTP(w, r.WithContext(newContext))
	})
}
