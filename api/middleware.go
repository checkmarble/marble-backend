package api

import (
	"context"
	"fmt"
	. "marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
	"strings"
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

func (api *API) enforceRoleMiddleware(requiredRole Role) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			ctx := r.Context()
			creds := utils.CredentialsFromCtx(ctx)
			grantedRole := creds.Role
			if grantedRole < requiredRole {
				api.logger.WarnCtx(ctx, "Token role not allowed for this endpoint")
				http.Error(w, "", http.StatusUnauthorized)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}
