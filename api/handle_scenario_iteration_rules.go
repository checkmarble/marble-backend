package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/operators"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
)

type APIScenarioIterationRule struct {
	ID                   string          `json:"id"`
	ScenarioIterationID  string          `json:"scenarioIterationId"`
	DisplayOrder         int             `json:"displayOrder"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Formula              json.RawMessage `json:"formula"`
	FormulaAstExpression json.RawMessage `json:"formula_ast_expression"`
	ScoreModifier        int             `json:"scoreModifier"`
	CreatedAt            time.Time       `json:"createdAt"`
}

func NewAPIScenarioIterationRule(rule models.Rule) (APIScenarioIterationRule, error) {
	var formulaAstExpression []byte
	formula, err := rule.Formula.MarshalJSON()
	if err != nil {
		return APIScenarioIterationRule{}, fmt.Errorf("unable to marshal formula: %w", err)
	}

	formulaAstExpression = nil
	if rule.FormulaAstExpression != nil {
		formulaAst, err := dto.AdaptNodeDto(*rule.FormulaAstExpression)
		formulaAstExpression, err = json.Marshal(formulaAst)
		if err != nil {
			formulaAstExpression = nil
		}
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
		logger := api.logger.With(slog.String("scenarioIterationId", input.ScenarioIterationID), slog.String("orgID", orgID))

		options := &utils.PtrToOptions{OmitZero: true}
		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		rules, err := usecase.ListScenarioIterationRules(ctx, orgID, models.GetScenarioIterationRulesFilters{
			ScenarioIterationID: utils.PtrTo(input.ScenarioIterationID, options),
		})
		if err != nil {
			logger.ErrorCtx(ctx, "Error listing rules:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		apiRules := make([]APIScenarioIterationRule, len(rules))
		for i, rule := range rules {
			apiRule, err := NewAPIScenarioIterationRule(rule)
			if err != nil {
				logger.ErrorCtx(ctx, "Error marshalling API scenario iteration rule:\n"+err.Error())
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			apiRules[i] = apiRule
		}

		err = json.NewEncoder(w).Encode(apiRules)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

func (api *API) CreateScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateScenarioIterationRuleInput)
		logger := api.logger.With(slog.String("scenarioIterationId", input.Body.ScenarioIterationID), slog.String("orgID", orgID))

		formula, err := operators.UnmarshalOperatorBool(input.Body.Formula)
		if err != nil {
			logger.WarnCtx(ctx, "Could not unmarshal formula:\n"+err.Error())
			http.Error(w, "", http.StatusUnprocessableEntity)
			return
		}

		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		rule, err := usecase.CreateScenarioIterationRule(ctx, orgID, models.CreateRuleInput{
			ScenarioIterationID: input.Body.ScenarioIterationID,
			DisplayOrder:        input.Body.DisplayOrder,
			Name:                input.Body.Name,
			Description:         input.Body.Description,
			Formula:             formula,
			ScoreModifier:       input.Body.ScoreModifier,
		})
		if err != nil {
			logger.ErrorCtx(ctx, "Error creating scenario iteration rule:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		apiRule, err := NewAPIScenarioIterationRule(rule)
		if err != nil {
			logger.ErrorCtx(ctx, "Error marshalling API scenario iteration rule:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiRule)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
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
		logger := api.logger.With(slog.String("ruleId", input.RuleID), slog.String("orgID", orgID))

		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		rule, err := usecase.GetScenarioIterationRule(ctx, orgID, input.RuleID)
		if errors.Is(err, models.NotFoundInRepositoryError) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			// Could not execute request
			logger.ErrorCtx(ctx, "Could not get scenario iteration rule: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		apiRule, err := NewAPIScenarioIterationRule(rule)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not marshall API scenario iteration rule: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiRule)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

func (api *API) UpdateScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.UpdateScenarioIterationRuleInput)
		logger := api.logger.With(slog.String("ruleId", input.RuleID), slog.String("orgID", orgID))

		updateRuleInput := models.UpdateRuleInput{
			ID:            input.RuleID,
			DisplayOrder:  input.Body.DisplayOrder,
			Name:          input.Body.Name,
			Description:   input.Body.Description,
			ScoreModifier: input.Body.ScoreModifier,
		}

		if input.Body.Formula != nil {
			formula, err := operators.UnmarshalOperatorBool(*input.Body.Formula)
			if err != nil {
				logger.ErrorCtx(ctx, "Could not unmarshal formula:\n"+err.Error())
				http.Error(w, "", http.StatusUnprocessableEntity)
				return
			}
			updateRuleInput.Formula = &formula
		}

		usecase := api.usecases.NewScenarioIterationRuleUsecase()
		updatedRule, err := usecase.UpdateScenarioIterationRule(ctx, orgID, updateRuleInput)
		if errors.Is(err, models.ErrScenarioIterationNotDraft) {
			http.Error(w, "", http.StatusForbidden)
			return
		} else if errors.Is(err, models.NotFoundInRepositoryError) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Error updating rule:\n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		apiRule, err := NewAPIScenarioIterationRule(updatedRule)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not marshall API scenario iteration rule: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiRule)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}
