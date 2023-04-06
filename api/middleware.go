package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// AuthCtx sets the organization ID in the context from the authorization header
func (a *API) authCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// read token from headers
		bearerToken := r.Header.Get("Authorization")        // looks like 'Bearer 12345'
		token := strings.TrimPrefix(bearerToken, "Bearer ") // looks like '12345'

		// Find org from token
		orgID, err := a.app.GetOrganizationIDFromToken(ctx, token)

		// If error, stop processing here
		if err != nil {
			http.Error(w, fmt.Errorf("invalid token").Error(), http.StatusUnauthorized)
			return
		}

		// Else, add org ID to the context
		log.Printf("token %v matched organization # %v\n", token, orgID)

		ctx = context.WithValue(ctx, contextKeyOrgID, orgID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

var ErrOrgNotInContext = fmt.Errorf("organization ID not found in request context")

func orgIDFromCtx(ctx context.Context) (id string, err error) {

	orgID, found := ctx.Value(contextKeyOrgID).(string)

	if !found {
		return "", ErrOrgNotInContext
	}

	return orgID, nil
}
