package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleGetEditorIdentifiers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		scenarioId, err := requiredUuidUrlParam(r, "scenarioId")
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

		PresentModel(w, struct {
			DatabaseAccessors []dto.NodeDto `json:"database_accessors"`
			PayloadAccessors  []dto.NodeDto `json:"payload_accessors"`
		}{
			DatabaseAccessors: databaseNodes,
			PayloadAccessors:  payloadbaseNodes,
		})
	}
}

func (api *API) handleGetEditorOperators() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		usecase := api.UsecasesWithCreds(r).AstExpressionUsecase()
		result := usecase.EditorOperators()

		var functions []dto.FuncAttributesDto

		for _, attributes := range result.OperatorAccessors {
			functions = append(functions, dto.AdaptFuncAttributesDto(attributes))
		}
		PresentModel(w, struct {
			OperatorAccessors []dto.FuncAttributesDto `json:"operators_accessors"`
		}{
			OperatorAccessors: functions,
		})
	}
}
