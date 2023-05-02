package api

import (
	"net/http"
	"time"

	"github.com/ggicci/httpin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/exp/slog"
)

var HARD_CODED_API_TOKEN_API = "12345"
var HARD_CODED_API_TOKEN_USER = "67890"
var HARD_CODED_ORG_ID = "12345"
var SIGNING_ALGO = jwt.SigningMethodRS256

const TOKEN_LIFETIME_MINUTES = 30

type GetNewAccessTokenInput struct {
	Credentials Credentials `in:"body=json"`
}

func (api *API) handleGetAccessToken() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*GetNewAccessTokenInput)
		creds := input.Credentials

		orgID, err := api.app.GetOrganizationIDFromToken(ctx, creds.RefreshToken)
		if err != nil && creds.RefreshToken != HARD_CODED_API_TOKEN_API && creds.RefreshToken != HARD_CODED_API_TOKEN_USER {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		logger := api.logger.With(slog.String("orgID", orgID))

		var tokenType TokenType
		if creds.RefreshToken == HARD_CODED_API_TOKEN_API || orgID != "" {
			tokenType = ApiToken
		} else {
			tokenType = UserToken
			orgID = HARD_CODED_ORG_ID
		}

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

		token := jwt.NewWithClaims(SIGNING_ALGO, claims)

		privateKey, _, err := api.signingSecretAccessor.ReadSigningSecrets(ctx)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not read private key:\n"+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		}

		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not create jwt:\n"+err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Write([]byte(tokenString))
		w.WriteHeader(http.StatusOK)

	}
}
