package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (api *API) handlePostFirebaseIdToken() http.HandlerFunc {

	return func(w http.ResponseWriter, request *http.Request) {

		// api key from header
		apiKey := strings.TrimSpace(request.Header.Get("X-API-Key"))

		// token from header
		idToken, err := ParseAuthorizationBearerHeader(request.Header)
		if err != nil {
			http.Error(w, fmt.Sprintf("Authorization header: %s", err.Error()), http.StatusBadRequest)
			return
		}

		usecase := api.usecases.MarbleTokenUseCase()
		marbleToken, expirationTime, err := usecase.NewMarbleToken(request.Context(), apiKey, idToken)
		if presentError(w, err) {
			return
		}

		PresentModel(w, struct {
			AccessToken string    `json:"access_token"`
			TokenType   string    `json:"token_type"`
			ExpiresAt   time.Time `json:"expires_at"`
		}{
			AccessToken: marbleToken,
			TokenType:   "Bearer",
			ExpiresAt:   *expirationTime,
		})
	}

}
