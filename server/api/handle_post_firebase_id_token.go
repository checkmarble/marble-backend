package api

import (
	"fmt"
	"marble/marble-backend/server/middlewares"
	"marble/marble-backend/utils"
	"net/http"
	"time"
)

func (api *API) handlePostFirebaseIdToken() http.HandlerFunc {

	return func(w http.ResponseWriter, request *http.Request) {

		// api key from header
		key := middlewares.ParseApiKeyHeader(request.Header)

		// token from header
		bearerToken, err := utils.ParseAuthorizationBearerHeader(request.Header)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
			return
		}

		context := request.Context()

		usecase := api.usecases.NewMarbleTokenUseCase()
		marbleToken, expirationTime, err := usecase.NewMarbleToken(context, key, bearerToken)
		if err != nil {
			err = middlewares.WrapErrInUnAuthorizedError(err)
		}
		if utils.PresentError(w, request, err) {
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
