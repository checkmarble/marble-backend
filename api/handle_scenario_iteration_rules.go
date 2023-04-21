package api

import (
	"context"
	"encoding/json"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type ScenarioIterationRuleAppInterface interface {
	GetScenarioIterationRules(ctx context.Context, organizationID string, scenarioIterationID string) ([]app.Rule, error)
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

func (api *API) handleGetScenarioIterationRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		scenarioIterationID := chi.URLParam(r, "scenarioIterationID")

		rules, err := api.app.GetScenarioIterationRules(ctx, orgID, scenarioIterationID)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenario_iteration(id: %s) rules: %w", scenarioIterationID, err).Error(), http.StatusInternalServerError)
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

type CreateScenarioIterationRuleInput struct {
	DisplayOrder  int             `json:"displayOrder"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Formula       json.RawMessage `json:"formula"`
	ScoreModifier int             `json:"scoreModifier"`
}

func (api *API) handlePostScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		scenarioIterationID := chi.URLParam(r, "scenarioIterationID")

		requestData := &CreateScenarioIterationRuleInput{}
		err = json.NewDecoder(r.Body).Decode(requestData)
		if err != nil {
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		formula, err := operators.UnmarshalOperatorBool(requestData.Formula)
		if err != nil {
			http.Error(w, fmt.Errorf("could not unmarshal formula: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		rule, err := api.app.CreateScenarioIterationRule(ctx, orgID, app.CreateRuleInput{
			ScenarioIterationID: scenarioIterationID,
			DisplayOrder:        requestData.DisplayOrder,
			Name:                requestData.Name,
			Description:         requestData.Description,
			Formula:             formula,
			ScoreModifier:       requestData.ScoreModifier,
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

func (api *API) handleGetScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		ruleID := chi.URLParam(r, "ruleID")

		rule, err := api.app.GetScenarioIterationRule(ctx, orgID, ruleID)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting rule(id: %s): %w", ruleID, err).Error(), http.StatusInternalServerError)
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

type UpdateScenarioIterationRuleInput struct {
	DisplayOrder  *int             `json:"displayOrder,omitempty"`
	Name          *string          `json:"name,omitempty"`
	Description   *string          `json:"description,omitempty"`
	Formula       *json.RawMessage `json:"formula,omitempty"`
	ScoreModifier *int             `json:"scoreModifier,omitempty"`
}

func (api *API) handlePutScenarioIterationRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		ruleID := chi.URLParam(r, "ruleID")

		requestData := &UpdateScenarioIterationRuleInput{}
		err = json.NewDecoder(r.Body).Decode(requestData)
		if err != nil {
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		updateRuleInput := app.UpdateRuleInput{
			ID:            ruleID,
			DisplayOrder:  requestData.DisplayOrder,
			Name:          requestData.Name,
			Description:   requestData.Description,
			ScoreModifier: requestData.ScoreModifier,
		}

		if requestData.Formula != nil {
			formula, err := operators.UnmarshalOperatorBool(*requestData.Formula)
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
