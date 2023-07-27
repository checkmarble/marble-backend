package api

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/operators"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
)

type APIScenarioIterationRule struct {
	ID                   string          `json:"id"`
	ScenarioIterationID  string          `json:"scenarioIterationId"`
	DisplayOrder         int             `json:"displayOrder"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Formula              json.RawMessage `json:"formula"`
	FormulaAstExpression *dto.NodeDto    `json:"formula_ast_expression"`
	ScoreModifier        int             `json:"scoreModifier"`
	CreatedAt            time.Time       `json:"createdAt"`
}

func NewAPIScenarioIterationRule(rule models.Rule) (APIScenarioIterationRule, error) {

	var formula []byte

	if rule.Formula != nil {
		var err error
		formula, err = (*rule.Formula).MarshalJSON()
		if err != nil {
			return APIScenarioIterationRule{}, fmt.Errorf("unable to marshal formula: %w", err)
		}
	}

	var formulaAstExpression *dto.NodeDto
	if rule.FormulaAstExpression != nil {
		nodeDto, err := dto.AdaptNodeDto(*rule.FormulaAstExpression)
		if err != nil {
			return APIScenarioIterationRule{}, nil
		}
		formulaAstExpression = &nodeDto
	}

	return APIScenarioIterationRule{
		ID:                   rule.ID,
		ScenarioIterationID:  rule.ScenarioIterationID,
		DisplayOrder:         rule.DisplayOrder,
		Name:                 rule.Name,
		Description:          rule.Description,
		Formula:              formula,
		FormulaAstExpression: formulaAstExpression,
		ScoreModifier:        rule.ScoreModifier,
		CreatedAt:            rule.CreatedAt,
	}, nil
}

type ListScenarioIterationRulesInput struct {
	ScenarioIterationID string `in:"query=scenarioIterationId"`
}

func (api *API) ListScenarioIterationRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*ListScenarioIterationRulesInput)

		options := &utils.PtrToOptions{OmitZero: true}
		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		rules, err := usecase.ListScenarioIterationRules(ctx, orgID, models.GetScenarioIterationRulesFilters{
			ScenarioIterationID: utils.PtrTo(input.ScenarioIterationID, options),
		})
		if presentError(w, r, err) {
			return
		}

		apiRules, err := utils.MapErr(rules, NewAPIScenarioIterationRule)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, apiRules)
	}
}

func adaptCreateRuleInput(body dto.CreateScenarioIterationRuleInputBody) (models.CreateRuleInput, error) {

	createRuleInput := models.CreateRuleInput{
		ScenarioIterationID:  body.ScenarioIterationID,
		DisplayOrder:         body.DisplayOrder,
		Name:                 body.Name,
		Description:          body.Description,
		Formula:              nil,
		FormulaAstExpression: nil,
		ScoreModifier:        body.ScoreModifier,
	}

	if body.Formula != nil {
		formula, err := operators.UnmarshalOperatorBool(body.Formula)
		if err != nil {
			return models.CreateRuleInput{}, fmt.Errorf("could not unmarshal formula: %w %w", err, models.BadParameterError)
		}

		createRuleInput.Formula = formula
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

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateScenarioIterationRuleInput)

		createInputRule, err := adaptCreateRuleInput(*input.Body)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		rule, err := usecase.CreateScenarioIterationRule(ctx, orgID, createInputRule)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := NewAPIScenarioIterationRule(rule)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, apiRule)
	}
}

type GetScenarioIterationRuleInput struct {
	RuleID string `in:"path=ruleID"`
}

func (api *API) GetScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioIterationRuleInput)

		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		rule, err := usecase.GetScenarioIterationRule(ctx, orgID, input.RuleID)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := NewAPIScenarioIterationRule(rule)
		if presentError(w, r, err) {
			return
		}
		PresentModel(w, apiRule)
	}
}

func adaptUpdateScenarioIterationRule(ruleId string, body dto.UpdateScenarioIterationRuleBody) (models.UpdateRuleInput, error) {

	updateRuleInput := models.UpdateRuleInput{
		ID:                   ruleId,
		DisplayOrder:         body.DisplayOrder,
		Name:                 body.Name,
		Description:          body.Description,
		Formula:              nil,
		FormulaAstExpression: nil,
		ScoreModifier:        body.ScoreModifier,
	}

	if body.Formula != nil {
		formula, err := operators.UnmarshalOperatorBool(*body.Formula)
		if err != nil {
			return models.UpdateRuleInput{}, fmt.Errorf("could not unmarshal formula: %w %w", err, models.BadParameterError)
		}

		updateRuleInput.Formula = &formula
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

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.UpdateScenarioIterationRuleInput)

		updateRuleInput, err := adaptUpdateScenarioIterationRule(input.RuleID, *input.Body)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		updatedRule, err := usecase.UpdateScenarioIterationRule(ctx, orgID, updateRuleInput)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := NewAPIScenarioIterationRule(updatedRule)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, apiRule)
	}
}
