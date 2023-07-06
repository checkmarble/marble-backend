package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/utils"
	"net/http"
)

func (api *API) handleGetEditorIdentifiers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		scenarioId, err := requiredUuidUrlParam(r, "scenarioID")
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).AstExpressionUsecase()
		result, err := usecase.EditorIdentifiers(scenarioId)

		if presentError(w, r, err) {
			return
		}

		nodes, err := utils.MapErr(result.DataAccessors, dto.AdaptNodeDto)
		if presentError(w, r, err) {
			return
		}
		PresentModel(w, struct {
			DataAccessors []dto.NodeDto `json:"data_accessors"`
		}{
			DataAccessors: nodes,
		})
	}
}
