package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtKey = []byte("MY_SECRET_KEY")
var HARD_CODED_API_TOKEN_API = "12345"
var HARD_CODED_API_TOKEN_USER = "67890"
var HARD_CODED_ORG_ID = "12345"
var TOKEN_LIFETIME_MINUTES = 30
var SIGNING_ALGO = jwt.SigningMethodRS256

type Credentials struct {
	RefreshToken string `json:"refresh_token"`
}

type Role int

const (
	READER Role = iota
	BUILDER
	PUBLISHER
	ADMIN
)

func (r Role) String() string {
	return [...]string{"READER", "BUILDER", "PUBLISHER", "ADMIN"}[r]
}
func RoleFromString(s string) Role {
	switch s {
	case "READER":
		return READER
	case "BUILDER":
		return BUILDER
	case "PUBLISHER":
		return PUBLISHER
	case "ADMIN":
		return ADMIN
	}
	return READER
}

type TokenType string

const (
	ApiToken      TokenType = "API"
	UserToken     TokenType = "USER"
	InternalToken TokenType = "INTERNAL"
)

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

		if creds.RefreshToken != HARD_CODED_API_TOKEN_API && creds.RefreshToken != HARD_CODED_API_TOKEN_USER {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var tokenType TokenType
		if creds.RefreshToken == HARD_CODED_API_TOKEN_API {
			tokenType = ApiToken
		} else {
			tokenType = UserToken
		}

		// Create the Claims
		expirationTime := time.Now().Add(time.Duration(TOKEN_LIFETIME_MINUTES) * time.Minute)
		claims := &Claims{
			OrganizationId: HARD_CODED_ORG_ID,
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
