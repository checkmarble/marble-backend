package api

import (
	"fmt"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

type ListScenarioIterationRulesInput struct {
	ScenarioIterationId string `in:"query=scenarioIterationId"`
}

func (api *API) ListScenarioIterationRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*ListScenarioIterationRulesInput)

		options := &utils.PtrToOptions{OmitZero: true}
		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		rules, err := usecase.ListScenarioIterationRules(ctx, organizationId, models.GetScenarioIterationRulesFilters{
			ScenarioIterationId: utils.PtrTo(input.ScenarioIterationId, options),
		})
		if presentError(w, r, err) {
			return
		}

		apiRules, err := utils.MapErr(rules, dto.AdaptScenarioIterationRuleDto)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, apiRules)
	}
}

func adaptCreateRuleInput(body dto.CreateScenarioIterationRuleInputBody) (models.CreateRuleInput, error) {

	createRuleInput := models.CreateRuleInput{
		ScenarioIterationId:  body.ScenarioIterationId,
		DisplayOrder:         body.DisplayOrder,
		Name:                 body.Name,
		Description:          body.Description,
		FormulaAstExpression: nil,
		ScoreModifier:        body.ScoreModifier,
	}

	if body.FormulaAstExpression != nil {
		node, err := dto.AdaptASTNode(*body.FormulaAstExpression)
		if err != nil {
			return models.CreateRuleInput{}, fmt.Errorf("could not adapt formula ast expression: %w %w", err, models.BadParameterError)
		}
		createRuleInput.FormulaAstExpression = &node
	}

	return createRuleInput, nil
}

func (api *API) CreateScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateScenarioIterationRuleInput)

		createInputRule, err := adaptCreateRuleInput(*input.Body)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		rule, err := usecase.CreateScenarioIterationRule(ctx, organizationId, createInputRule)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := dto.AdaptScenarioIterationRuleDto(rule)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, struct {
			Rule dto.ScenarioIterationRuleDto `json:"rule"`
		}{
			Rule: apiRule,
		})
	}
}

type GetScenarioIterationRuleInput struct {
	RuleID string `in:"path=ruleID"`
}

func (api *API) GetScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioIterationRuleInput)

		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		rule, err := usecase.GetScenarioIterationRule(ctx, organizationId, input.RuleID)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := dto.AdaptScenarioIterationRuleDto(rule)
		if presentError(w, r, err) {
			return
		}
		PresentModel(w, struct {
			Rule dto.ScenarioIterationRuleDto `json:"rule"`
		}{
			Rule: apiRule,
		})
	}
}

func adaptUpdateScenarioIterationRule(ruleId string, body dto.UpdateScenarioIterationRuleBody) (models.UpdateRuleInput, error) {

	updateRuleInput := models.UpdateRuleInput{
		Id:                   ruleId,
		DisplayOrder:         body.DisplayOrder,
		Name:                 body.Name,
		Description:          body.Description,
		FormulaAstExpression: nil,
		ScoreModifier:        body.ScoreModifier,
	}

	if body.FormulaAstExpression != nil {
		node, err := dto.AdaptASTNode(*body.FormulaAstExpression)
		if err != nil {
			return models.UpdateRuleInput{}, fmt.Errorf("could not adapt formula ast expression: %w %w", err, models.BadParameterError)
		}
		updateRuleInput.FormulaAstExpression = &node
	}

	return updateRuleInput, nil
}

func (api *API) UpdateScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.UpdateScenarioIterationRuleInput)

		updateRuleInput, err := adaptUpdateScenarioIterationRule(input.RuleID, *input.Body)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		updatedRule, scenarioValidation, err := usecase.UpdateScenarioIterationRule(ctx, organizationId, updateRuleInput)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := dto.AdaptScenarioIterationRuleDto(updatedRule)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, struct {
			Rule               dto.ScenarioIterationRuleDto `json:"rule"`
			ScenarioValidation dto.ScenarioValidationDto    `json:"scenario_validation"`
		}{
			Rule:               apiRule,
			ScenarioValidation: dto.AdaptScenarioValidationDto(scenarioValidation),
		})
	}
}
