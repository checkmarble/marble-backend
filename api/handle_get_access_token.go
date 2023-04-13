package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtKey = []byte("MY_SECRET_KEY")
var HARD_CODED_API_TOKEN = "12345"
var HARD_CODED_ORG_ID = "12345"
var TOKEN_LIFETIME_MINUTES = 30
var SIGNING_ALGO = jwt.SigningMethodHS256

type Credentials struct {
	RefreshToken string `json:"refresh_token"`
}

// We add jwt.RegisteredClaims as an embedded type, to provide fields like expiry time
type Claims struct {
	OrganizationId string `json:"organization_id"`
	Type           string `json:"type"`
	Role           string `json:"role"`
	jwt.RegisteredClaims
}

func (api *API) handleGetAccessToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ctx := r.Context()

		var creds Credentials
		// Get the JSON body and decode into credentials
		err := json.NewDecoder(r.Body).Decode(&creds)
		if err != nil {
			// If the structure of the body is wrong, return an HTTP error
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if creds.RefreshToken != HARD_CODED_API_TOKEN {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Create the Claims
		// Declare the expiration time of the token
		// here, we have kept it as 5 minutes
		expirationTime := time.Now().Add(time.Duration(TOKEN_LIFETIME_MINUTES) * time.Minute)
		// Create the JWT claims, which includes the username and expiry time
		claims := &Claims{
			OrganizationId: HARD_CODED_ORG_ID,
			Type:           "API",
			Role:           "ADMIN",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationTime),
				Issuer:    "marble",
			},
		}

		// Declare the token with the algorithm used for signing, and the claims
		token := jwt.NewWithClaims(SIGNING_ALGO, claims)
		// Create the JWT string
		tokenString, err := token.SignedString(jwtKey)
		if err != nil {
			// If there is an error in creating the JWT return an internal server error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Write([]byte(tokenString))
		w.WriteHeader(http.StatusOK)

	}
}
