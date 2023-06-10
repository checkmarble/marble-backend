package api

import (
	"encoding/json"
	"errors"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
)

type APIScenario struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	TriggerObjectType string    `json:"triggerObjectType"`
	CreatedAt         time.Time `json:"createdAt"`
	LiveVersionID     *string   `json:"liveVersionId,omitempty"`
	ScenarioType      string    `json:"scenarioType"`
}

func NewAPIScenario(scenario models.Scenario) APIScenario {
	return APIScenario{
		ID:                scenario.ID,
		Name:              scenario.Name,
		Description:       scenario.Description,
		TriggerObjectType: scenario.TriggerObjectType,
		CreatedAt:         scenario.CreatedAt,
		LiveVersionID:     scenario.LiveVersionID,
		ScenarioType:      scenario.ScenarioType.String(),
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
		filters := ctx.Value(httpin.Input).(*dto.ListScenarioInput)
		usecase := api.usecases.NewScenarioUsecase()
		scenarios, err := usecase.ListScenarios(ctx, orgID, filters.ToFilters())

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

func (api *API) CreateScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateScenarioInput)
		logger := api.logger.With(slog.String("orgID", orgID))

		usecase := api.usecases.NewScenarioUsecase()
		scenario, err := usecase.CreateScenario(ctx, orgID, models.CreateScenarioInput{
			Name:              input.Body.Name,
			Description:       input.Body.Description,
			TriggerObjectType: input.Body.TriggerObjectType,
			ScenarioType:      models.ScenarioTypeFrom(input.Body.ScenarioType),
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

		usecase := api.usecases.NewScenarioUsecase()
		scenario, err := usecase.GetScenario(ctx, orgID, input.ScenarioID)
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

func (api *API) UpdateScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.UpdateScenarioInput)
		logger := api.logger.With(slog.String("orgID", orgID), slog.String("scenarioID", input.ScenarioID))

		usecase := api.usecases.NewScenarioUsecase()
		scenario, err := usecase.UpdateScenario(ctx, orgID, models.UpdateScenarioInput{
			ID:          input.ScenarioID,
			Name:        input.Body.Name,
			Description: input.Body.Description,
		})
		if errors.Is(err, models.NotFoundInRepositoryError) {
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
