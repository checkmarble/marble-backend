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

type ScenarioIterationAppInterface interface {
	GetScenarioIterations(ctx context.Context, orgID string, scenarioID string) ([]app.ScenarioIteration, error)
	CreateScenarioIteration(ctx context.Context, orgID string, scenarioIteration app.CreateScenarioIterationInput) (app.ScenarioIteration, error)
	GetScenarioIteration(ctx context.Context, orgID string, scenarioIterationID string) (app.ScenarioIteration, error)
	UpdateScenarioIteration(ctx context.Context, organizationID string, rule app.UpdateScenarioIterationInput) (app.ScenarioIteration, error)
}

type APIScenarioIterationBody struct {
	TriggerCondition     json.RawMessage            `json:"triggerCondition"`
	Rules                []APIScenarioIterationRule `json:"rules,omitempty"`
	ScoreReviewThreshold int                        `json:"scoreReviewThreshold"`
	ScoreRejectThreshold int                        `json:"scoreRejectThreshold"`
}

type APIScenarioIteration struct {
	ID         string    `json:"id"`
	ScenarioID string    `json:"scenarioId"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func NewAPIScenarioIteration(si app.ScenarioIteration) APIScenarioIteration {
	return APIScenarioIteration{
		ID:         si.ID,
		ScenarioID: si.ScenarioID,
		Version:    si.Version,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
	}
}

type APIScenarioIterationWithBody struct {
	APIScenarioIteration
	Body APIScenarioIterationBody `json:"body"`
}

func NewAPIScenarioIterationWithBody(si app.ScenarioIteration) (APIScenarioIterationWithBody, error) {
	triggerConditionBytes, err := si.Body.TriggerCondition.MarshalJSON()
	if err != nil {
		return APIScenarioIterationWithBody{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
	}

	body := APIScenarioIterationBody{
		TriggerCondition:     triggerConditionBytes,
		ScoreReviewThreshold: si.Body.ScoreReviewThreshold,
		ScoreRejectThreshold: si.Body.ScoreRejectThreshold,
	}
	for _, rule := range si.Body.Rules {
		apiRule, err := NewAPIScenarioIterationRule(rule)
		if err != nil {
			return APIScenarioIterationWithBody{}, fmt.Errorf("could not create new api scenario iteration rule: %w", err)
		}
		body.Rules = append(body.Rules, apiRule)
	}

	return APIScenarioIterationWithBody{
		APIScenarioIteration: NewAPIScenarioIteration(si),
		Body:                 body,
	}, nil
}

func (a *API) handleGetScenarioIterations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		scenarioID := chi.URLParam(r, "scenarioID")

		scenarioIterations, err := a.app.GetScenarioIterations(ctx, orgID, scenarioID)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenario(id: %s) iterations: %w", scenarioID, err).Error(), http.StatusInternalServerError)
			return
		}

		var apiScenarioIterations []APIScenarioIteration
		for _, si := range scenarioIterations {
			apiScenarioIterations = append(apiScenarioIterations, NewAPIScenarioIteration(si))
		}

		err = json.NewEncoder(w).Encode(apiScenarioIterations)
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type CreateScenarioIterationBody struct {
	TriggerCondition     json.RawMessage                    `json:"triggerCondition"`
	Rules                []CreateScenarioIterationRuleInput `json:"rules"`
	ScoreReviewThreshold int                                `json:"scoreReviewThreshold"`
	ScoreRejectThreshold int                                `json:"scoreRejectThreshold"`
}

type CreateScenarioIterationInput struct {
	Body *CreateScenarioIterationBody `json:"body"`
}

func (a *API) handlePostScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		scenarioID := chi.URLParam(r, "scenarioID")

		requestData := &CreateScenarioIterationInput{}
		err = json.NewDecoder(r.Body).Decode(requestData)
		if err != nil {
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		triggerCondition, err := operators.UnmarshalOperatorBool(requestData.Body.TriggerCondition)
		if err != nil {
			http.Error(w, fmt.Errorf("could not unmarshal trigger condition: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		createScenarioIterationInput := app.CreateScenarioIterationInput{
			ScenarioID: scenarioID,
			Body: app.CreateScenarioIterationBody{
				TriggerCondition:     triggerCondition,
				ScoreReviewThreshold: requestData.Body.ScoreReviewThreshold,
				ScoreRejectThreshold: requestData.Body.ScoreRejectThreshold,
			},
		}
		for _, rule := range requestData.Body.Rules {
			formula, err := operators.UnmarshalOperatorBool(rule.Formula)
			if err != nil {
				http.Error(w, fmt.Errorf("could not unmarshal formula: %w", err).Error(), http.StatusUnprocessableEntity)
				return
			}
			createScenarioIterationInput.Body.Rules = append(createScenarioIterationInput.Body.Rules, app.CreateRuleInput{
				DisplayOrder:  rule.DisplayOrder,
				Name:          rule.Name,
				Description:   rule.Description,
				Formula:       formula,
				ScoreModifier: rule.ScoreModifier,
			})
		}

		si, err := a.app.CreateScenarioIteration(ctx, orgID, createScenarioIterationInput)
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting scenarios: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		apiScenarioIterationWithBody, err := NewAPIScenarioIterationWithBody(si)
		if err != nil {
			http.Error(w, fmt.Errorf("could not create new api scenario iteration: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiScenarioIterationWithBody)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (a *API) handleGetScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		scenarioIterationID := chi.URLParam(r, "scenarioIterationID")

		si, err := a.app.GetScenarioIteration(ctx, orgID, scenarioIterationID)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenarioIterationID(id: %s): %w", scenarioIterationID, err).Error(), http.StatusInternalServerError)
			return
		}

		apiScenarioIterationWithBody, err := NewAPIScenarioIterationWithBody(si)
		if err != nil {
			http.Error(w, fmt.Errorf("could not create new api scenario iteration: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiScenarioIterationWithBody)
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type UpdateScenarioIterationInput struct {
	Body *struct {
		TriggerCondition     *json.RawMessage `json:"triggerCondition"`
		ScoreReviewThreshold *int             `json:"scoreReviewThreshold"`
		ScoreRejectThreshold *int             `json:"scoreRejectThreshold"`
	} `json:"body"`
}

func (a *API) handlePutScenarioIteration() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		scenarioIterationID := chi.URLParam(r, "scenarioIterationID")

		requestData := &UpdateScenarioIterationInput{}
		err = json.NewDecoder(r.Body).Decode(requestData)
		if err != nil {
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		if requestData.Body == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		updateScenarioIterationInput := app.UpdateScenarioIterationInput{
			ID: scenarioIterationID,
			Body: &app.UpdateScenarioIterationBody{
				ScoreReviewThreshold: requestData.Body.ScoreReviewThreshold,
				ScoreRejectThreshold: requestData.Body.ScoreRejectThreshold,
			},
		}

		if requestData.Body.TriggerCondition != nil {
			triggerCondition, err := operators.UnmarshalOperatorBool(*requestData.Body.TriggerCondition)
			if err != nil {
				http.Error(w, fmt.Errorf("could not unmarshal triggerCondition: %w", err).Error(), http.StatusUnprocessableEntity)
				return
			}
			updateScenarioIterationInput.Body.TriggerCondition = &triggerCondition
		}

		updatedSI, err := a.app.UpdateScenarioIteration(ctx, orgID, updateScenarioIterationInput)
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting scenario iteration: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		apiRule, err := NewAPIScenarioIterationWithBody(updatedSI)
		if err != nil {
			http.Error(w, fmt.Errorf("could not create new api scenario iteration: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(apiRule)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
