package api

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

func (api *API) handleAvailableFunctions() http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {

		functions := make(map[string]dto.FuncAttributesDto)

		for f, attributes := range ast.FuncAttributesMap {
			if f == ast.FUNC_CONSTANT {
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

type PostRunAstExpression struct {
	Body struct {
		Expression  *dto.NodeDto    `json:"expression"`
		Payload     json.RawMessage `json:"payload"`
		PayloadType string          `json:"payload_type"`
	} `in:"body=json"`
}

type RunAstExpressionResultDto struct {
	Evaluation dto.NodeEvaluationDto `json:"evaluation"`
}

func (api *API) handleDryRunAstExpression() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		input := ctx.Value(httpin.Input).(*PostRunAstExpression)

		expression, err := dto.AdaptASTNode(*input.Body.Expression)
		if err != nil {
			presentError(w, r, fmt.Errorf("invalid Expression: %w", models.BadParameterError))
			return
		}

		usecase := api.UsecasesWithCreds(r).AstExpressionUsecase()
		evaluation, err := usecase.DryRun(expression, input.Body.PayloadType, input.Body.Payload)
		if presentError(w, r, err) {
			return
		}

		if presentError(w, r, err) {
			return
		}

		PresentModel(w, RunAstExpressionResultDto{
			Evaluation: dto.AdaptNodeEvaluationDto(evaluation),
		})
	}
}

type PatchRuleWithAstExpression struct {
	Body struct {
		Expression *dto.NodeDto `json:"expression"`
		RuleId     string       `json:"rule_id"`
	} `in:"body=json"`
}

func (api *API) handleSaveRuleWithAstExpression() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := r.Context().Value(httpin.Input).(*PatchRuleWithAstExpression)

		if err := utils.ValidateUuid(input.Body.RuleId); err != nil {
			presentError(w, r, err)
		}

		expression, err := dto.AdaptASTNode(*input.Body.Expression)
		if err != nil {
			presentError(w, r, fmt.Errorf("invalid Expression: %w %w", err, models.BadParameterError))
			return
		}

		usecase := api.UsecasesWithCreds(r).AstExpressionUsecase()
		err = usecase.SaveRuleWithAstExpression(input.Body.RuleId, expression)
		if err != nil {
			presentError(w, r, fmt.Errorf("invalid Expression: %w %w", err, models.BadParameterError))
		}
		PresentNothing(w)
	}
}
