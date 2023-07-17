package api

import (
	"fmt"
	"net/http"
	"time"
)

func (api *API) handlePostFirebaseIdToken() http.HandlerFunc {

	return func(w http.ResponseWriter, request *http.Request) {

		// api key from header
		key := ParseApiKeyHeader(request.Header)

		// token from header
		bearerToken, err := ParseAuthorizationBearerHeader(request.Header)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
			return
		}

		usecase := api.UsecasesWithCreds(request).NewMarbleTokenUseCase()
		marbleToken, expirationTime, err := usecase.NewMarbleToken(key, bearerToken)
		if err != nil {
			err = wrapErrInUnAuthorizedError(err)
		}
		if presentError(w, request, err) {
			return
		}

		PresentModel(w, struct {
			AccessToken string    `json:"access_token"`
			TokenType   string    `json:"token_type"`
			ExpiresAt   time.Time `json:"expires_at"`
		}{
			AccessToken: marbleToken,
			TokenType:   "Bearer",
			ExpiresAt:   expirationTime,
		})
	}

}
