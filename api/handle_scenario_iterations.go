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
}

type APIScenarioIterationBody struct {
	TriggerCondition operators.OperatorBool `json:"triggerCondition"`
	// Rules                []Rule                 `json:"rules"`
	ScoreReviewThreshold int `json:"scoreReviewThreshold"`
	ScoreRejectThreshold int `json:"scoreRejectThreshold"`
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

func NewAPIScenarioIterationWithBody(si app.ScenarioIteration) APIScenarioIterationWithBody {
	body := APIScenarioIterationBody{
		TriggerCondition:     si.Body.TriggerCondition,
		ScoreReviewThreshold: si.Body.ScoreReviewThreshold,
		ScoreRejectThreshold: si.Body.ScoreRejectThreshold,
	}
	// for _, rule := range si.Body.Rules {
	// 	body.Rules = append(body.Rules, rule)
	// }

	return APIScenarioIterationWithBody{
		APIScenarioIteration: NewAPIScenarioIteration(si),
		Body:                 body,
	}
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

type CreateScenarioIterationInput struct {
	Body app.ScenarioIterationBody `json:"body"`
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
			// Could not parse JSON
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		scenario, err := a.app.CreateScenarioIteration(ctx, orgID, app.CreateScenarioIterationInput{
			ScenarioID: scenarioID,
			Body:       requestData.Body,
		})
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting scenarios: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIScenarioIterationWithBody(scenario))
		if err != nil {
			// Could not encode JSON
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

		err = json.NewEncoder(w).Encode(NewAPIScenarioIterationWithBody(si))
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
