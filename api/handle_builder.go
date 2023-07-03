package api

import (
	"net/http"
)

func (api *API) handleGetBuilderIdentifier() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		usecase := api.UsecasesWithCreds(r).AstExpressionUsecase()
		result, err := usecase.Identifiers()

		if presentError(w, r, err) {
			return
		}

		PresentModel(w, result)
	}
}
