package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models/ast"
)

func (api *API) handleAvailableFunctions() http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {

		functions := make(map[string]dto.FuncAttributesDto)

		for f, attributes := range ast.FuncAttributesMap {
			if f == ast.FUNC_CONSTANT || f == ast.FUNC_UNDEFINED {
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
