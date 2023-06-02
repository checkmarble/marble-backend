package api

import (
	"context"
	"encoding/json"
	"errors"
	"marble/marble-backend/app"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
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
	LiveVersionID     *string   `json:"liveVersionId,omitempty"`
}

func NewAPIScenario(scenario app.Scenario) APIScenario {
	return APIScenario{
		ID:                scenario.ID,
		Name:              scenario.Name,
		Description:       scenario.Description,
		TriggerObjectType: scenario.TriggerObjectType,
		CreatedAt:         scenario.CreatedAt,
		LiveVersionID:     scenario.LiveVersionID,
	}
}

func (api *API) ListScenarios() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		logger := api.logger.With(slog.String("orgID", orgID))

		scenarios, err := api.app.ListScenarios(ctx, orgID)

		if presentError(w, r, err) {
			return
		}

		apiScenarios := make([]APIScenario, len(scenarios))
		for i, scenario := range scenarios {
			apiScenarios[i] = NewAPIScenario(scenario)
		}

		err = json.NewEncoder(w).Encode(apiScenarios)
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
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

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*CreateScenarioInput)
		logger := api.logger.With(slog.String("orgID", orgID))

		scenario, err := api.app.CreateScenario(ctx, orgID, app.CreateScenarioInput{
			Name:              input.Body.Name,
			Description:       input.Body.Description,
			TriggerObjectType: input.Body.TriggerObjectType,
		})
		if err != nil {
			logger.ErrorCtx(ctx, "Error creating scenario: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIScenario(scenario))
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
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

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioInput)
		logger := api.logger.With(slog.String("orgID", orgID), slog.String("scenarioID", input.ScenarioID))

		scenario, err := api.app.GetScenario(ctx, orgID, input.ScenarioID)
		if errors.Is(err, models.NotFoundError) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Error getting scenario: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIScenario(scenario))
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
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

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*UpdateScenarioInput)
		logger := api.logger.With(slog.String("orgID", orgID), slog.String("scenarioID", input.ScenarioID))

		scenario, err := api.app.UpdateScenario(ctx, orgID, app.UpdateScenarioInput{
			ID:          input.ScenarioID,
			Name:        input.Body.Name,
			Description: input.Body.Description,
		})
		if errors.Is(err, app.ErrNotFoundInRepository) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Error updating scenario: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIScenario(scenario))
		if err != nil {
			logger.ErrorCtx(ctx, "Could not encode response JSON: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}
