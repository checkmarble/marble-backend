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

		databaseNodes, err := utils.MapErr(result.DatabaseAccessors, dto.AdaptNodeDto)
		if presentError(w, r, err) {
			return
		}
		payloadbaseNodes, err := utils.MapErr(result.PayloadAccessors, dto.AdaptNodeDto)
		if presentError(w, r, err) {
			return
		}
		customListNodes, err := utils.MapErr(result.CustomListAccessors, dto.AdaptNodeDto)
		if presentError(w, r, err) {
			return
		}
		PresentModel(w, struct {
			DatabaseAccessors   []dto.NodeDto `json:"database_accessors"`
			PayloadAccessors    []dto.NodeDto `json:"payload_accessors"`
			CustomListAccessors []dto.NodeDto `json:"custom_list_accessors"`
		}{
			DatabaseAccessors:   databaseNodes,
			PayloadAccessors:    payloadbaseNodes,
			CustomListAccessors: customListNodes,
		})
	}
}
