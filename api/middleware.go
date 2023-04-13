package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var HARD_CODED_PUBLIC_KEY = []byte("MY_SECRET_KEY")

var VALIDATION_ALGO = jwt.SigningMethodRS256

// AuthCtx sets the organization ID in the context from the authorization header
func (a *API) jwtValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.Split(r.Header.Get("Authorization"), "Bearer ")
		if len(authHeader) != 2 {
			fmt.Println("Malformed token")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Malformed Token"))
		} else {
			jwtToken := authHeader[1]
			token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
				method, ok := token.Method.(*jwt.SigningMethodRSA)
				if !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				if method != VALIDATION_ALGO {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(HARD_CODED_PUBLIC_KEY), nil
			})

			if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
				ctx := context.WithValue(r.Context(), "claims", claims)
				// Access context values in handlers like this
				// props, _ := r.Context().Value("props").(jwt.MapClaims)
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				fmt.Println(err)
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Unauthorized"))
			}
		}

	})
}

func (a *API) authCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		claims, ok := ctx.Value("claims").(jwt.MapClaims)
		if !ok {
			log.Println("claims not found in context")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}
		organizationId, ok := claims["organization_id"].(string)
		if !ok {
			log.Println("organization_id not found in claims")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}

		ctx = context.WithValue(ctx, contextKeyOrgID, organizationId)
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
