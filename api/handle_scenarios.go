package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
)

type ScenarioAppInterface interface {
	ListScenarios(ctx context.Context, organizationID string) ([]app.Scenario, error)
	CreateScenario(ctx context.Context, organizationID string, scenario app.CreateScenarioInput) (app.Scenario, error)
	UpdateScenario(ctx context.Context, organizationID string, scenario app.UpdateScenarioInput) (app.Scenario, error)
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

func (api *API) ListScenarios() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		scenarios, err := api.app.ListScenarios(ctx, orgID)
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

type CreateScenarioBody struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	TriggerObjectType string `json:"triggerObjectType"`
}

type CreateScenarioInput struct {
	Body *CreateScenarioBody `in:"body=json"`
}

func (api *API) CreateScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*CreateScenarioInput)

		scenario, err := api.app.CreateScenario(ctx, orgID, app.CreateScenarioInput{
			Name:              input.Body.Name,
			Description:       input.Body.Description,
			TriggerObjectType: input.Body.TriggerObjectType,
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

type GetScenarioInput struct {
	ScenarioID string `in:"path=scenarioID"`
}

func (api *API) GetScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioInput)

		scenario, err := api.app.GetScenario(ctx, orgID, input.ScenarioID)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenario(id: %s): %w", input.ScenarioID, err).Error(), http.StatusInternalServerError)
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

type UpdateScenarioBody struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type UpdateScenarioInput struct {
	ScenarioID string              `in:"path=scenarioID"`
	Body       *UpdateScenarioBody `in:"body=json"`
}

func (api *API) UpdateScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*UpdateScenarioInput)

		scenario, err := api.app.UpdateScenario(ctx, orgID, app.UpdateScenarioInput{
			ID:          input.ScenarioID,
			Name:        input.Body.Name,
			Description: input.Body.Description,
		})
		if errors.Is(err, app.ErrNotFoundInRepository) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting scenario(id: %s): %w", input.ScenarioID, err).Error(), http.StatusInternalServerError)
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
