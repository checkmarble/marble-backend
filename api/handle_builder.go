package api

import (
	"marble/marble-backend/utils"
	"net/http"
)

func (api *API) handleGetBuilderIdentifier() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		credentials := utils.MustCredentialsFromCtx(r.Context())

		usecase := api.usecases.AstExpressionUsecase(credentials)
		result, err := usecase.Identifiers()

		// var runtimeErrorDto string
		// if errors.Is(err, ast_eval.ErrRuntimeExpression) {
		// 	runtimeErrorDto = err.Error()
		// }

		if presentError(w, r, err) {
			return
		}

		PresentModel(w, result)
	}
}
