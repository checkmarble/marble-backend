package api

import (
	"fmt"
	"net/http"
	"time"
)

func (api *API) handlePostFirebaseIdToken() http.HandlerFunc {

	return func(w http.ResponseWriter, request *http.Request) {

		// api key from header
		apiKey := ParseApiKeyHeader(request.Header)

		// token from header
		idToken, err := ParseAuthorizationBearerHeader(request.Header)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
			return
		}

		context := request.Context()

		usecase := api.usecases.MarbleTokenUseCase()
		marbleToken, expirationTime, err := usecase.NewMarbleToken(context, apiKey, idToken)
		if err != nil {
			err = wrapErrInUnAuthorizedError(err)
		}
		if presentError(context, api.logger, w, err) {
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
