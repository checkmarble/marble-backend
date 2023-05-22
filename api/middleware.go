package api

import (
	"context"
	"fmt"
	. "marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
	"strings"

	"golang.org/x/exp/slices"
)

func ParseAuthorizationBearerHeader(header http.Header) (string, error) {
	authorization := header.Get("Authorization")
	if authorization == "" {
		return "", nil
	}

	authHeader := strings.Split(header.Get("Authorization"), "Bearer ")
	if len(authHeader) != 2 {
		return "", fmt.Errorf("Malformed Token")
	}

	return authHeader[1], nil
}

// AuthCtx sets the organization ID in the context from the authorization header
func (api *API) jwtValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		jwtToken, err := ParseAuthorizationBearerHeader(r.Header)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
			return
		}

		usecase := api.usecases.MarbleTokenUseCase()
		creds, err := usecase.ValidateMarbleToken(jwtToken)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		ctxWithToken := context.WithValue(r.Context(), utils.ContextKeyCredentials, creds)
		next.ServeHTTP(w, r.WithContext(ctxWithToken))
	})
}

func (api *API) enforcePermissionMiddleware(permission Permission) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			ctx := r.Context()
			creds := utils.CredentialsFromCtx(ctx)
			allowed := slices.Contains(creds.Role.Permissions(), permission)

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
