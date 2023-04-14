package api

import (
	"context"
	"encoding/json"
	"fmt"
	"marble/marble-backend/app"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type ScenarioAppInterface interface {
	GetScenarios(ctx context.Context, organizationID string) ([]app.Scenario, error)
	CreateScenario(ctx context.Context, organizationID string, scenario app.CreateScenarioInput) (app.Scenario, error)

	GetScenario(ctx context.Context, organizationID string, scenarioID string) (app.Scenario, error)
}

type APIScenario struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	TriggerObjectType string    `json:"triggerObjectType"`
	CreatedAt         time.Time `json:"createdAt"`
	IsLive            bool      `json:"isLive"`
}

func NewAPIScenario(scenario app.Scenario) APIScenario {
	return APIScenario{
		ID:                scenario.ID,
		Name:              scenario.Name,
		Description:       scenario.Description,
		TriggerObjectType: scenario.TriggerObjectType,
		CreatedAt:         scenario.CreatedAt,
		IsLive:            scenario.LiveVersion != nil,
	}
}

func (a *API) handleGetScenarios() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		scenarios, err := a.app.GetScenarios(ctx, orgID)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenarios: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		apiScenarios := make([]APIScenario, len(scenarios))
		for i, scenario := range scenarios {
			apiScenarios[i] = NewAPIScenario(scenario)
		}

		err = json.NewEncoder(w).Encode(apiScenarios)
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type CreateScenarioInput struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	TriggerObjectType string `json:"triggerObjectType"`
}

func (a *API) handlePostScenarios() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		requestData := &CreateScenarioInput{}
		err = json.NewDecoder(r.Body).Decode(requestData)
		if err != nil {
			// Could not parse JSON
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		scenario, err := a.app.CreateScenario(ctx, orgID, app.CreateScenarioInput{
			Name:              requestData.Name,
			Description:       requestData.Description,
			TriggerObjectType: requestData.TriggerObjectType,
		})
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting scenarios: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIScenario(scenario))
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (a *API) handleGetScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		scenarioID := chi.URLParam(r, "scenarioID")

		scenario, err := a.app.GetScenario(ctx, orgID, scenarioID)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenario(id: %s): %w", scenarioID, err).Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIScenario(scenario))
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
