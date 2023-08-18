package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/models/ast"
	"net/http"
)

func (api *API) handleAvailableFunctions() http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {

		functions := make(map[string]dto.FuncAttributesDto)

		for f, attributes := range ast.FuncAttributesMap {
			if f == ast.FUNC_CONSTANT || f == ast.FUNC_UNKNOWN {
				continue
			}
			functions[attributes.AstName] = dto.AdaptFuncAttributesDto(attributes)
		}

		PresentModel(w, struct {
			Functions map[string]dto.FuncAttributesDto `json:"functions"`
		}{
			Functions: functions,
		})
	}
}
