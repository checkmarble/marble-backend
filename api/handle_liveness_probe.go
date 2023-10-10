package api

import (
	"net/http"
)

func (api *API) handleLivenessProbe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		PresentModel(w, struct {
			Mood string `json:"mood"`
		}{
			Mood: "Feu flammes !",
		})
	}
}
