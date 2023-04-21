package api

import (
	"context"
	"encoding/json"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
)

type ScenarioIterationRuleAppInterface interface {
	GetScenarioIterationRules(ctx context.Context, organizationID string, filters app.GetScenarioIterationRulesFilters) ([]app.Rule, error)
	CreateScenarioIterationRule(ctx context.Context, organizationID string, rule app.CreateRuleInput) (app.Rule, error)
	GetScenarioIterationRule(ctx context.Context, organizationID string, ruleID string) (app.Rule, error)
	UpdateScenarioIterationRule(ctx context.Context, organizationID string, rule app.UpdateRuleInput) (app.Rule, error)
}

type APIScenarioIterationRule struct {
	ID            string          `json:"id"`
	DisplayOrder  int             `json:"displayOrder"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Formula       json.RawMessage `json:"formula"`
	ScoreModifier int             `json:"scoreModifier"`
	CreatedAt     time.Time       `json:"createdAt"`
}

func NewAPIScenarioIterationRule(rule app.Rule) (APIScenarioIterationRule, error) {
	formula, err := rule.Formula.MarshalJSON()
	if err != nil {
		return APIScenarioIterationRule{}, fmt.Errorf("unable to marshal formula: %w", err)
	}

	return APIScenarioIterationRule{
		ID:            rule.ID,
		DisplayOrder:  rule.DisplayOrder,
		Name:          rule.Name,
		Description:   rule.Description,
		Formula:       formula,
		ScoreModifier: rule.ScoreModifier,
		CreatedAt:     rule.CreatedAt,
	}, nil
}

type GetScenarioIterationRulesInput struct {
	ScenarioIterationID string `in:"query=scenarioIterationID"`
}

func (api *API) handleGetScenarioIterationRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioIterationRulesInput)

		options := &utils.PtrToOptions{OmitZero: true}
		rules, err := api.app.GetScenarioIterationRules(ctx, orgID, app.GetScenarioIterationRulesFilters{
			ScenarioIterationID: utils.PtrTo(input.ScenarioIterationID, options),
		})
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenario_iteration rules: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		apiRules := make([]APIScenarioIterationRule, len(rules))
		for i, rule := range rules {
			apiRule, err := NewAPIScenarioIterationRule(rule)
			if err != nil {
				http.Error(w, fmt.Errorf("could not create new api scenario iteration rule: %w", err).Error(), http.StatusInternalServerError)
				return
			}
			apiRules[i] = apiRule
		}

		err = json.NewEncoder(w).Encode(apiRules)
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type PostScenarioIterationRuleInputBody struct {
	ScenarioIterationID string          `json:"scenarioIterationId"`
	DisplayOrder        int             `json:"displayOrder"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	Formula             json.RawMessage `json:"formula"`
	ScoreModifier       int             `json:"scoreModifier"`
}

type PostScenarioIterationRuleInput struct {
	Body *PostScenarioIterationRuleInputBody `in:"body=json"`
}

func (api *API) handlePostScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*PostScenarioIterationRuleInput)

		formula, err := operators.UnmarshalOperatorBool(input.Body.Formula)
		if err != nil {
			http.Error(w, fmt.Errorf("could not unmarshal formula: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		rule, err := api.app.CreateScenarioIterationRule(ctx, orgID, app.CreateRuleInput{
			ScenarioIterationID: input.Body.ScenarioIterationID,
			DisplayOrder:        input.Body.DisplayOrder,
			Name:                input.Body.Name,
			Description:         input.Body.Description,
			Formula:             formula,
			ScoreModifier:       input.Body.ScoreModifier,
		})
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting rule: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		apiRule, err := NewAPIScenarioIterationRule(rule)
		if err != nil {
			http.Error(w, fmt.Errorf("could not create new api scenario iteration rule: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiRule)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type GetScenarioIterationRuleInput struct {
	RuleID string `in:"path=ruleID"`
}

func (api *API) handleGetScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioIterationRuleInput)

		rule, err := api.app.GetScenarioIterationRule(ctx, orgID, input.RuleID)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting rule(id: %s): %w", input.RuleID, err).Error(), http.StatusInternalServerError)
			return
		}

		apiRule, err := NewAPIScenarioIterationRule(rule)
		if err != nil {
			http.Error(w, fmt.Errorf("could not create new api scenario iteration rule: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiRule)
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type UpdateScenarioIterationRuleBody struct {
	DisplayOrder  *int             `json:"displayOrder,omitempty"`
	Name          *string          `json:"name,omitempty"`
	Description   *string          `json:"description,omitempty"`
	Formula       *json.RawMessage `json:"formula,omitempty"`
	ScoreModifier *int             `json:"scoreModifier,omitempty"`
}

type PutScenarioIterationRuleInput struct {
	RuleID string                           `in:"path=ruleID"`
	Body   *UpdateScenarioIterationRuleBody `in:"body=json"`
}

func (api *API) handlePutScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*PutScenarioIterationRuleInput)

		updateRuleInput := app.UpdateRuleInput{
			ID:            input.RuleID,
			DisplayOrder:  input.Body.DisplayOrder,
			Name:          input.Body.Name,
			Description:   input.Body.Description,
			ScoreModifier: input.Body.ScoreModifier,
		}

		if input.Body.Formula != nil {
			formula, err := operators.UnmarshalOperatorBool(*input.Body.Formula)
			if err != nil {
				http.Error(w, fmt.Errorf("could not unmarshal formula: %w", err).Error(), http.StatusUnprocessableEntity)
				return
			}
			updateRuleInput.Formula = &formula
		}

		updatedRule, err := api.app.UpdateScenarioIterationRule(ctx, orgID, updateRuleInput)
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting rule: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		apiRule, err := NewAPIScenarioIterationRule(updatedRule)
		if err != nil {
			http.Error(w, fmt.Errorf("could not create new api scenario iteration rule: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiRule)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
