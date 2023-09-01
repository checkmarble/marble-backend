package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/utils"
	"net/http"
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

		databaseNodes, err := utils.MapErr(result.DatabaseAccessors, dto.AdaptIdentifierDto)
		if presentError(w, r, err) {
			return
		}
		payloadbaseNodes, err := utils.MapErr(result.PayloadAccessors, dto.AdaptIdentifierDto)
		if presentError(w, r, err) {
			return
		}
		customListNodes, err := utils.MapErr(result.CustomListAccessors, dto.AdaptIdentifierDto)
		if presentError(w, r, err) {
			return
		}
		aggregatorNodes, err := utils.MapErr(result.AggregatorAccessors, dto.AdaptIdentifierDto)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, struct {
			DatabaseAccessors   []dto.IdentifierDto `json:"database_accessors"`
			PayloadAccessors    []dto.IdentifierDto `json:"payload_accessors"`
			CustomListAccessors []dto.IdentifierDto `json:"custom_list_accessors"`
			AggregatorAccessors []dto.IdentifierDto `json:"aggregator_accessors"`
		}{
			DatabaseAccessors:   databaseNodes,
			PayloadAccessors:    payloadbaseNodes,
			CustomListAccessors: customListNodes,
			AggregatorAccessors: aggregatorNodes,
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
