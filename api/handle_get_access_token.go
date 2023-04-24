package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var HARD_CODED_API_TOKEN_API = "12345"
var HARD_CODED_API_TOKEN_USER = "67890"
var HARD_CODED_ORG_ID = "12345"
var TOKEN_LIFETIME_MINUTES = 30
var SIGNING_ALGO = jwt.SigningMethodRS256

func (api *API) handleGetAccessToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var creds Credentials
		// Get the JSON body and decode into credentials
		err := json.NewDecoder(r.Body).Decode(&creds)
		if err != nil || creds.RefreshToken == "" {
			// If the structure of the body is wrong, return an HTTP error
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Find org from token
		orgID, err := api.app.GetOrganizationIDFromToken(ctx, creds.RefreshToken)
		if err != nil && creds.RefreshToken != HARD_CODED_API_TOKEN_API && creds.RefreshToken != HARD_CODED_API_TOKEN_USER {
			log.Println("No org found for token")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var tokenType TokenType
		if creds.RefreshToken == HARD_CODED_API_TOKEN_API || orgID != "" {
			tokenType = ApiToken
		} else {
			tokenType = UserToken
			orgID = HARD_CODED_ORG_ID
		}

		// Create the Claims
		expirationTime := time.Now().Add(time.Duration(TOKEN_LIFETIME_MINUTES) * time.Minute)
		claims := &Claims{
			OrganizationId: orgID,
			Type:           string(tokenType),
			Role:           "ADMIN",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationTime),
				Issuer:    "marble",
			},
		}

		// Declare the token with the algorithm used for signing, and the claims
		token := jwt.NewWithClaims(SIGNING_ALGO, claims)
		// Create the JWT string

		privateKey, _, err := api.signingSecretAccessor.ReadSigningSecrets(ctx)
		if err != nil {
			log.Printf("Could not read private key, %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
		}

		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			// If there is an error in creating the JWT return an internal server error
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Write([]byte(tokenString))
		w.WriteHeader(http.StatusOK)

	}
}
