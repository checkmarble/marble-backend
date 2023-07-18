package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval/evaluate"
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

type PostValidateAstExpression struct {
	Body struct {
		Expression *dto.NodeDto `json:"expression"`
	} `in:"body=json"`
}

func (api *API) handleValidateAstExpression() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*PostValidateAstExpression)

		expression, err := dto.AdaptASTNode(*input.Body.Expression)
		if err != nil {
			presentError(w, r, fmt.Errorf("invalid Expression: %w", models.BadParameterError))
			return
		}

		usecase := api.UsecasesWithCreds(r).AstExpressionUsecase()
		allErrors := usecase.Validate(expression)

		var validationErrorsDto = utils.Map(allErrors, func(err error) string {
			return err.Error()
		})

		if validationErrorsDto == nil {
			validationErrorsDto = []string{}
		}

		expressionDto, err := dto.AdaptNodeDto(expression)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, struct {
			Expression       dto.NodeDto `json:"expression"`
			ValidationErrors []string    `json:"validation_errors"`
		}{
			Expression:       expressionDto,
			ValidationErrors: validationErrorsDto,
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
	Result       any    `json:"result"`
	RuntimeError string `json:"runtime_error"`
}

func (api *API) handleRunAstExpression() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		input := ctx.Value(httpin.Input).(*PostRunAstExpression)
		logger := api.logger

		expression, err := dto.AdaptASTNode(*input.Body.Expression)
		if err != nil {
			presentError(w, r, fmt.Errorf("invalid Expression: %w", models.BadParameterError))
			return
		}

		organizationUsecase := api.usecases.NewOrganizationUseCase()

		dataModel, err := organizationUsecase.GetDataModel(api.UsecasesWithCreds(r).OrganizationIdOfContext)
		if presentError(w, r, err) {
			return
		}

		tables := dataModel.Tables
		table, ok := tables[models.TableName(input.Body.PayloadType)]
		if !ok {
			logger.ErrorCtx(ctx, "Table not found in data model for organization")
			http.Error(w, "", http.StatusNotFound)
			return
		}

		payload, err := app.ParseToDataModelObject(table, input.Body.Payload)
		if presentError(w, r, err) {
			return
		}
		usecase := api.UsecasesWithCreds(r).AstExpressionUsecase()
		result, err := usecase.Run(expression, payload)

		var runtimeErrorDto string
		if errors.Is(err, evaluate.ErrRuntimeExpression) {
			runtimeErrorDto = err.Error()
			err = nil
		}

		if presentError(w, r, err) {
			return
		}

		PresentModel(w, RunAstExpressionResultDto{
			Result:       result,
			RuntimeError: runtimeErrorDto,
		})
	}
}

type PatchIterationRuleWithAstExpression struct {
	Body struct {
		Expression          *dto.NodeDto `json:"expression"`
		ScenarioIterationId string       `json:"scenario_iteration_id"`
	} `in:"body=json"`
}

func (api *API) handleSaveIterationWithAstExpression() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := r.Context().Value(httpin.Input).(*PatchIterationRuleWithAstExpression)

		if err := utils.ValidateUuid(input.Body.ScenarioIterationId); err != nil {
			presentError(w, r, err)
		}

		expression, err := dto.AdaptASTNode(*input.Body.Expression)
		if err != nil {
			presentError(w, r, fmt.Errorf("invalid Expression: %w", models.BadParameterError))
			return
		}

		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).AstExpressionUsecase()
		usecase.SaveIterationWithAstExpression(expression, input.Body.ScenarioIterationId)
	}
}
